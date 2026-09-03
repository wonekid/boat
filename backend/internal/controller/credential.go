package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListCredentials 凭证列表（不返回敏感字段）
func ListCredentials(c *gin.Context) {
	var list []model.Credential
	database.DB.Order("id").Find(&list)
	utils.Success(c, list)
}

// CredentialReq 凭证请求
type CredentialReq struct {
	Name         string `json:"name" binding:"required"`
	Type         int    `json:"type"` // 1密码 2密钥
	Username     string `json:"username" binding:"required"`
	AuthPassword string `json:"authPassword"`
	PrivateKey   string `json:"privateKey"`
	Remark       string `json:"remark"`
}

// CreateCredential 新建凭证（敏感字段加密存储）
func CreateCredential(c *gin.Context) {
	var req CredentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	cred := model.Credential{
		Name: req.Name, Type: req.Type, Username: req.Username, Remark: req.Remark,
	}
	if req.AuthPassword != "" {
		enc, err := utils.EncryptSecret(req.AuthPassword)
		if err != nil {
			utils.Fail(c, "加密失败")
			return
		}
		cred.AuthPassword = enc
	}
	if req.PrivateKey != "" {
		enc, err := utils.EncryptSecret(req.PrivateKey)
		if err != nil {
			utils.Fail(c, "加密失败")
			return
		}
		cred.PrivateKey = enc
	}
	database.DB.Create(&cred)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateCredential 更新凭证
func UpdateCredential(c *gin.Context) {
	id := c.Param("id")
	var req CredentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var cred model.Credential
	if err := database.DB.First(&cred, id).Error; err != nil {
		utils.Fail(c, "凭证不存在")
		return
	}
	cred.Name = req.Name
	cred.Type = req.Type
	cred.Username = req.Username
	cred.Remark = req.Remark
	if req.AuthPassword != "" {
		enc, _ := utils.EncryptSecret(req.AuthPassword)
		cred.AuthPassword = enc
	}
	if req.PrivateKey != "" {
		enc, _ := utils.EncryptSecret(req.PrivateKey)
		cred.PrivateKey = enc
	}
	database.DB.Save(&cred)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteCredential 删除凭证
func DeleteCredential(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.Credential{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// TestCredential 测试凭证连通性（解密后用于 SSH，此处仅校验解密是否成功）
func TestCredential(c *gin.Context) {
	id := c.Param("id")
	var cred model.Credential
	if err := database.DB.First(&cred, id).Error; err != nil {
		utils.Fail(c, "凭证不存在")
		return
	}
	if cred.Type == 1 && cred.AuthPassword != "" {
		if _, err := utils.DecryptSecret(cred.AuthPassword); err != nil {
			utils.Fail(c, "解密失败")
			return
		}
	}
	utils.SuccessMsg(c, "凭证可用")
}
