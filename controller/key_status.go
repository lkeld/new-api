package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetKeyStatus(c *gin.Context) {
	var ch model.Channel
	model.DB.Where("base_url LIKE ?", "%55888%").First(&ch)
	if ch.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}

	full, _ := model.GetChannelById(ch.Id, true)
	if full == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}

	var quotaMap map[string]interface{}

	payload, err := json.Marshal(gin.H{"key": full.Key})
	if err == nil {
		req, reqErr := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "http://124.223.2.96:22335/api/query", bytes.NewBuffer(payload))
		if reqErr == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, doErr := service.GetHttpClient().Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				body, readErr := io.ReadAll(resp.Body)
				if readErr == nil {
					_ = json.Unmarshal(body, &quotaMap)
				}
			}
		}
	}

	var allocated int64
	model.DB.Model(&model.User{}).Select("COALESCE(SUM(quota), 0)").Scan(&allocated)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"quota":           quotaMap,
			"allocated_quota": allocated,
			"quota_per_unit":  common.QuotaPerUnit,
			"upstream_ok":     quotaMap != nil,
		},
	})
}
