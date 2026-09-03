package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Code     string `json:"code"` // 验证码（演示版可选）
}

// Login 登录
func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	if user.Status == 0 {
		utils.Fail(c, "账号已禁用")
		return
	}
	if !utils.PasswordVerify(req.Password, user.Password) {
		// 登录失败日志
		writeAudit(0, req.Username, "登录", "认证", c.ClientIP(), 0, "密码错误")
		utils.Fail(c, "密码错误")
		return
	}
	// 已开启 MFA：密码通过后返回临时令牌，待动态码校验
	if user.MFAEnabled {
		mfaToken, err := utils.GenerateMFAToken(user.ID, user.Username)
		if err != nil {
			utils.Fail(c, "MFA 令牌生成失败")
			return
		}
		writeAudit(user.ID, user.Username, "登录(待MFA)", "认证", c.ClientIP(), 1, "")
		utils.Success(c, gin.H{"mfaRequired": true, "mfaToken": mfaToken})
		return
	}
	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		utils.Fail(c, "令牌生成失败")
		return
	}
	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", &now)
	writeAudit(user.ID, user.Username, "登录成功", "认证", c.ClientIP(), 1, "")
	utils.Success(c, gin.H{
		"token": token,
		"user":  user,
	})
}

// MFAVerifyReq MFA 二次校验
type MFAVerifyReq struct {
	MFAToken string `json:"mfaToken" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// MFAVerify 密码通过后校验 TOTP 动态码，通过则签发正式令牌
func MFAVerify(c *gin.Context) {
	var req MFAVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	claims, err := utils.ParseToken(req.MFAToken)
	if err != nil || !claims.Mfa {
		utils.Fail(c, "MFA 令牌无效或已过期")
		return
	}
	var user model.User
	if err := database.DB.First(&user, claims.UserID).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	if !user.MFAEnabled || user.MFASecret == "" {
		utils.Fail(c, "该用户未开启 MFA")
		return
	}
	secret, err := utils.DecryptSecret(user.MFASecret)
	if err != nil {
		utils.Fail(c, "密钥解析失败")
		return
	}
	if !totp.Validate(req.Code, secret) {
		writeAudit(user.ID, user.Username, "MFA校验失败", "认证", c.ClientIP(), 0, "动态码错误")
		utils.Fail(c, "动态验证码错误")
		return
	}
	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		utils.Fail(c, "令牌生成失败")
		return
	}
	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", &now)
	writeAudit(user.ID, user.Username, "登录成功", "认证", c.ClientIP(), 1, "MFA通过")
	utils.Success(c, gin.H{"token": token, "user": user})
}

// MFASetup 生成 TOTP 密钥并返回 otpauth URL（开启前预览，写入但未启用）
func MFASetup(c *gin.Context) {
	uid, _ := middlewareUser(c)
	var user model.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "BoatOps", AccountName: user.Username})
	if err != nil {
		utils.Fail(c, "生成密钥失败")
		return
	}
	enc, err := utils.EncryptSecret(key.Secret())
	if err != nil {
		utils.Fail(c, "加密失败")
		return
	}
	// 暂存密钥，等待 Enable 校验通过后正式启用
	database.DB.Model(&user).Update("mfa_secret", enc)
	utils.Success(c, gin.H{"secret": key.Secret(), "otpauth": key.URL()})
}

// MFAEnableReq 启用 MFA
type MFAEnableReq struct {
	Code string `json:"code" binding:"required"`
}

// MFAEnable 用动态码确认启用 MFA
func MFAEnable(c *gin.Context) {
	uid, _ := middlewareUser(c)
	var req MFAEnableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	if user.MFASecret == "" {
		utils.Fail(c, "请先生成密钥")
		return
	}
	secret, err := utils.DecryptSecret(user.MFASecret)
	if err != nil {
		utils.Fail(c, "密钥解析失败")
		return
	}
	if !totp.Validate(req.Code, secret) {
		utils.Fail(c, "动态验证码错误")
		return
	}
	database.DB.Model(&user).Update("mfa_enabled", 1)
	utils.SuccessMsg(c, "MFA 已启用")
}

// MFADisableReq 关闭 MFA
type MFADisableReq struct {
	Code string `json:"code" binding:"required"`
}

// MFADisable 校验动态码后关闭 MFA 并清除密钥
func MFADisable(c *gin.Context) {
	uid, _ := middlewareUser(c)
	var req MFADisableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	if user.MFAEnabled {
		secret, _ := utils.DecryptSecret(user.MFASecret)
		if secret != "" && !totp.Validate(req.Code, secret) {
			utils.Fail(c, "动态验证码错误")
			return
		}
	}
	database.DB.Model(&user).Updates(map[string]interface{}{"mfa_enabled": 0, "mfa_secret": ""})
	utils.SuccessMsg(c, "MFA 已关闭")
}

// Profile 当前用户信息
func Profile(c *gin.Context) {
	uid, _ := middlewareUser(c)
	var user model.User
	if err := database.DB.Preload("Roles").Preload("Roles.Menus").First(&user, uid).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	// 组装权限菜单树（仅状态正常）
	utils.Success(c, user)
}

// ChangePwdReq 改密
type ChangePwdReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	uid, _ := middlewareUser(c)
	var req ChangePwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	if !utils.PasswordVerify(req.OldPassword, user.Password) {
		utils.Fail(c, "原密码错误")
		return
	}
	user.Password = utils.PasswordHash(req.NewPassword)
	database.DB.Save(&user)
	utils.SuccessMsg(c, "修改成功")
}

// Captcha 演示版图形验证码（返回文本，前端直接展示）
func Captcha(c *gin.Context) {
	code := RandomCode(4)
	// 生产环境应存 Redis 并校验；此处仅演示
	utils.Success(c, gin.H{"code": code})
}

// writeAudit 写审计日志
func writeAudit(userID uint, username, action, module, ip string, status int, detail string) {
	log := model.AuditLog{
		UserID:  userID,
		Username: username,
		Action:  action,
		Module:  module,
		IP:      ip,
		Status:  status,
		Detail:  detail,
	}
	database.DB.Create(&log)
}

// RandomCode 生成简单验证码
func RandomCode(n int) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	seed := time.Now().UnixNano()
	b := make([]byte, n)
	for i := range b {
		seed = seed*1103515245 + 12345
		b[i] = chars[(seed>>16)%int64(len(chars))]
	}
	return string(b)
}

var _ = http.StatusOK
