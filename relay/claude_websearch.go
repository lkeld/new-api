package relay

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const webSearchToolName = "web_search"

// claudeWebSearchToolInfo reports whether the incoming Claude request declares the
// Anthropic server-side web_search tool (type web_search_YYYYMMDD), and the per-request
// search cap it asks for (falling back to the configured default).
func claudeWebSearchToolInfo(req *dto.ClaudeRequest) (bool, int) {
	if req == nil || req.Tools == nil {
		return false, 0
	}
	tools, err := common.Any2Type[[]map[string]any](req.Tools)
	if err != nil {
		return false, 0
	}
	for _, t := range tools {
		typ, _ := t["type"].(string)
		if !strings.HasPrefix(typ, "web_search") {
			continue
		}
		maxUses := setting.WebSearchDefaultMaxUses
		if v, ok := t["max_uses"].(float64); ok && int(v) > 0 {
			maxUses = int(v)
		}
		return true, maxUses
	}
	return false, 0
}

// tryClaudeWebSearch runs the gateway-executed web_search loop when the request asks for
// the Anthropic web_search tool against a non-Anthropic (OpenAI-compatible) upstream that
// cannot run it natively. Returns (err, handled); when handled is false the caller
// continues the normal relay path unchanged.
func tryClaudeWebSearch(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest, adaptor channel.Adaptor) (*types.NewAPIError, bool) {
	if !setting.WebSearchEnabled || info.ApiType != constant.APITypeOpenAI {
		return nil, false
	}
	found, maxUses := claudeWebSearchToolInfo(req)
	if !found {
		return nil, false
	}
	return claudeWebSearchHelper(c, info, req, adaptor, maxUses), true
}

// claudeWebSearchHelper turns the gateway into the search agent: it exposes web_search to
// the model as a normal function, loops (non-streaming) running real Brave searches and
// feeding the results back until the model stops searching, then streams the final answer
// to the client through the standard OpenAI->Claude response path.
func claudeWebSearchHelper(c *gin.Context, info *relaycommon.RelayInfo, claudeReq *dto.ClaudeRequest, adaptor channel.Adaptor, maxUses int) *types.NewAPIError {
	oaiReq, err := service.ClaudeToOpenAIRequest(*claudeReq, info)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	// Give web_search a usable function schema + description so the model can call it.
	for i := range oaiReq.Tools {
		if oaiReq.Tools[i].Function.Name == webSearchToolName {
			oaiReq.Tools[i].Type = "function"
			oaiReq.Tools[i].Function.Description = "Search the web for current, real-world information. Returns a list of results (title, url, snippet). Call this whenever the answer depends on up-to-date or external facts."
			oaiReq.Tools[i].Function.Parameters = map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query.",
					},
				},
				"required": []string{"query"},
			}
		}
	}

	streamFalse := false
	searchCount := 0

	for iter := 0; iter < maxUses+2; iter++ {
		oaiReq.Stream = &streamFalse
		oaiResp, apiErr := webSearchUpstreamCall(c, info, adaptor, oaiReq)
		if apiErr != nil {
			return apiErr
		}
		if len(oaiResp.Choices) == 0 {
			break
		}
		msg := oaiResp.Choices[0].Message
		searchCalls := make([]dto.ToolCallRequest, 0)
		for _, tc := range msg.ParseToolCalls() {
			if tc.Function.Name == webSearchToolName {
				searchCalls = append(searchCalls, tc)
			}
		}
		if len(searchCalls) == 0 || searchCount >= maxUses {
			break // model is done searching -> stream the final answer
		}
		oaiReq.Messages = append(oaiReq.Messages, dto.Message{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
		})
		for _, tc := range searchCalls {
			toolMsg := dto.Message{Role: "tool", ToolCallId: tc.ID}
			toolMsg.SetStringContent(runBraveSearch(extractSearchQuery(tc.Function.Arguments)))
			oaiReq.Messages = append(oaiReq.Messages, toolMsg)
			searchCount++
			if searchCount >= maxUses {
				break
			}
		}
	}

	// Final answer: drop web_search so the model stops searching, then stream the reply
	// to the client through the normal OpenAI->Claude conversion (adaptor.DoResponse).
	oaiReq.Tools = stripWebSearchTool(oaiReq.Tools)
	if searchCount > 0 {
		// Steer the model to answer from the gathered results instead of re-entering its
		// tool-calling template. Some GLM endpoints otherwise leak the raw <|tool_calls|>
		// markup as text in streaming mode after a tool loop.
		oaiReq.Messages = append(oaiReq.Messages, dto.Message{
			Role:    "user",
			Content: "You now have web search results above. Write your final answer to my original request as plain natural-language text, and cite the source URLs you used. Do not output any tool calls, function calls, or special control tokens — only the natural-language answer.",
		})
	}
	streamTrue := true
	oaiReq.Stream = &streamTrue
	info.IsStream = true

	body, err := common.Marshal(oaiReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	respAny, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResp, _ := respAny.(*http.Response)
	if httpResp == nil {
		return types.NewOpenAIError(fmt.Errorf("nil upstream response"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if httpResp.StatusCode != http.StatusOK {
		return service.RelayErrorHandler(c.Request.Context(), httpResp, false)
	}
	usage, apiErr := adaptor.DoResponse(c, httpResp, info)
	if apiErr != nil {
		return apiErr
	}
	if searchCount > 0 {
		c.Set("claude_web_search_requests", searchCount)
	}
	if u, ok := usage.(*dto.Usage); ok {
		service.PostTextConsumeQuota(c, info, u, nil)
	}
	return nil
}

// webSearchUpstreamCall sends one non-streaming chat-completions request and parses it.
func webSearchUpstreamCall(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, oaiReq *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, *types.NewAPIError) {
	body, err := common.Marshal(oaiReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	respAny, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResp, _ := respAny.(*http.Response)
	if httpResp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("nil upstream response"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, service.RelayErrorHandler(c.Request.Context(), httpResp, false)
	}
	var oaiResp dto.OpenAITextResponse
	if err := common.DecodeJson(httpResp.Body, &oaiResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	return &oaiResp, nil
}

func extractSearchQuery(arguments string) string {
	var args struct {
		Query string `json:"query"`
	}
	_ = common.UnmarshalJsonStr(arguments, &args)
	return strings.TrimSpace(args.Query)
}

// runBraveSearch executes one search and formats the results as plain text for the model.
// Errors are returned as text (not failures) so a flaky search never aborts the turn.
func runBraveSearch(query string) string {
	if query == "" {
		return "Web search error: empty query."
	}
	results, err := service.BraveWebSearch(query, setting.WebSearchResultsPerQuery)
	if err != nil {
		return fmt.Sprintf("Web search error for %q: %v", query, err)
	}
	if len(results) == 0 {
		return fmt.Sprintf("No web results found for %q.", query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Web search results for %q:\n\n", query)
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s\nURL: %s\n%s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}
	return b.String()
}

func stripWebSearchTool(tools []dto.ToolCallRequest) []dto.ToolCallRequest {
	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, t := range tools {
		if t.Function.Name != webSearchToolName {
			out = append(out, t)
		}
	}
	return out
}
