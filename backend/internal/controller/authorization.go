package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListAuthorizations 授权列表（可按用户/类型筛选）
func ListAuthorizations(c *gin.Context) {
	userID := c.Query("userId")
	targetType := c.Query("targetType")
	q := database.DB.Model(&model.Authorization{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	var list []model.Authorization
	q.Order("id DESC").Find(&list)
	utils.Success(c, list)
}

// AuthReq 授权请求
type AuthReq struct {
	UserID     uint   `json:"userId" binding:"required"`
	TargetType string `json:"targetType" binding:"required"` // host|credential|hostGroup|credentialGroup
	TargetIDs  []uint `json:"targetIds"`
}

// CreateAuthorization 授权（用户-资源）
func CreateAuthorization(c *gin.Context) {
	var req AuthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var auths []model.Authorization
	for _, tid := range req.TargetIDs {
		auths = append(auths, model.Authorization{
			UserID: req.UserID, TargetType: req.TargetType, TargetID: tid,
		})
	}
	database.DB.CreateInBatches(auths, 50)
	utils.SuccessMsg(c, "授权成功")
}

// DeleteAuthorization 撤销授权
func DeleteAuthorization(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.Authorization{}, id)
	utils.SuccessMsg(c, "已撤销")
}
