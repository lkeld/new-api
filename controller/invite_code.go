package controller

import (
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllInviteCodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	codes, total, err := model.GetAllInviteCodes(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(codes)
	common.ApiSuccess(c, pageInfo)
}

func SearchInviteCodes(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	codes, total, err := model.SearchInviteCodes(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(codes)
	common.ApiSuccess(c, pageInfo)
}

func GetInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	code, err := model.GetInviteCodeById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": code})
}

func AddInviteCode(c *gin.Context) {
	ic := model.InviteCode{}
	if err := c.ShouldBindJSON(&ic); err != nil {
		common.ApiError(c, err)
		return
	}
	if utf8.RuneCountInString(ic.Name) == 0 || utf8.RuneCountInString(ic.Name) > 20 {
		common.ApiError(c, errors.New("邀请码名称长度必须在 1-20 之间 / name length must be 1-20"))
		return
	}
	if ic.Count <= 0 {
		common.ApiError(c, errors.New("生成数量必须大于 0 / count must be positive"))
		return
	}
	if ic.Count > 100 {
		common.ApiError(c, errors.New("一次最多生成 100 个 / at most 100 at a time"))
		return
	}
	if ic.MaxUses < 0 {
		common.ApiError(c, errors.New("最大使用次数不能为负 / max uses cannot be negative"))
		return
	}
	if ic.ExpiredTime != 0 && ic.ExpiredTime < common.GetTimestamp() {
		common.ApiError(c, errors.New("过期时间不能早于当前时间 / expiry cannot be in the past"))
		return
	}
	var codes []string
	for i := 0; i < ic.Count; i++ {
		code := common.GetUUID()
		clean := model.InviteCode{
			UserId:      c.GetInt("id"),
			Name:        ic.Name,
			Code:        code,
			MaxUses:     ic.MaxUses, // 0 = unlimited
			Status:      common.InviteCodeStatusEnabled,
			CreatedTime: common.GetTimestamp(),
			ExpiredTime: ic.ExpiredTime,
		}
		if err := clean.Insert(); err != nil {
			common.SysError("failed to insert invite code: " + err.Error())
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "创建邀请码失败 / failed to create", "data": codes})
			return
		}
		codes = append(codes, code)
	}
	recordManageAudit(c, "invite_code.create", map[string]interface{}{
		"name":     ic.Name,
		"count":    ic.Count,
		"max_uses": ic.MaxUses,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": codes})
}

func UpdateInviteCode(c *gin.Context) {
	statusOnly := c.Query("status_only")
	ic := model.InviteCode{}
	if err := c.ShouldBindJSON(&ic); err != nil {
		common.ApiError(c, err)
		return
	}
	clean, err := model.GetInviteCodeById(ic.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if statusOnly == "" {
		if ic.ExpiredTime != 0 && ic.ExpiredTime < common.GetTimestamp() {
			common.ApiError(c, errors.New("过期时间不能早于当前时间 / expiry cannot be in the past"))
			return
		}
		// If you add more fields, also update InviteCode.Update()
		clean.Name = ic.Name
		clean.MaxUses = ic.MaxUses
		clean.ExpiredTime = ic.ExpiredTime
	} else {
		clean.Status = ic.Status
	}
	if err := clean.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": clean})
}

func DeleteInviteCode(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteInviteCodeById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func DeleteInvalidInviteCode(c *gin.Context) {
	rows, err := model.DeleteInvalidInviteCodes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}
