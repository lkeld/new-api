package setting

// Gateway-executed web search (Anthropic web_search-compatible).
//
// When WebSearchEnabled is true and an incoming Claude (/v1/messages) request declares
// the server-side `web_search` tool, the gateway runs the searches ITSELF (via the Brave
// Search API) and feeds the results back to the upstream model. This lets Claude Code's
// web search work through non-Anthropic models (e.g. GLM-5.2) that have no native
// server-side search, while preserving Anthropic's web_search_tool_result + citations
// response shape so clients render sources exactly like real Claude.
//
// Settings are admin-tunable via the option API (keys mirror the variable names).
var (
	// WebSearchEnabled is the master toggle for the gateway-executed web_search tool.
	WebSearchEnabled = false
	// BraveSearchAPIKey is the Brave Search API subscription token (X-Subscription-Token).
	BraveSearchAPIKey = ""
	// WebSearchDefaultMaxUses caps how many searches one request may run when the tool
	// definition omits max_uses (mirrors Anthropic's per-request search cap).
	WebSearchDefaultMaxUses = 5
	// WebSearchResultsPerQuery is how many results to fetch from Brave per search.
	WebSearchResultsPerQuery = 5
)
