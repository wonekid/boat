package controller

import (
	"boat/internal/database"
	"boat/internal/casbin"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListUsers 用户列表
func ListUsers(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	status := c.Query("status")

	q := database.DB.Model(&model.User{})
	if keyword != "" {
		q = q.Where("username LIKE ? OR nickname LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []model.User
	q.Preload("Roles").Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}

// UserReq 创建/更新用户
type UserReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Status   int    `json:"status"`
	DeptID   uint   `json:"deptId"`
	RoleIDs  []uint `json:"roleIds"`
}

// CreateUser 新建用户
func CreateUser(c *gin.Context) {
	var req UserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var exist model.User
	if err := database.DB.Where("username = ?", req.Username).First(&exist).Error; err == nil {
		utils.Fail(c, "用户名已存在")
		return
	}
	user := model.User{
		Username: req.Username,
		Password: utils.PasswordHash(req.Password),
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   req.Status,
		DeptID:   req.DeptID,
	}
	if user.Password == "" {
		user.Password = utils.PasswordHash("123456")
	}
	database.DB.Create(&user)
	// 关联角色
	if len(req.RoleIDs) > 0 {
		var roles []model.Role
		database.DB.Find(&roles, req.RoleIDs)
		database.DB.Model(&user).Association("Roles").Replace(roles)
	}
	_ = casbin.SyncFromDB()
	uid, uname := middlewareUser(c)
	writeAudit(uid, uname, "创建用户", "用户管理", clientIP(c), 1, req.Username)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateUser 更新用户
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req UserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var user model.User
	if err := database.DB.First(&user, id).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	user.Nickname = req.Nickname
	user.Email = req.Email
	user.Phone = req.Phone
	user.Status = req.Status
	user.DeptID = req.DeptID
	if req.Password != "" {
		user.Password = utils.PasswordHash(req.Password)
	}
	database.DB.Save(&user)
	if len(req.RoleIDs) > 0 {
		var roles []model.Role
		database.DB.Find(&roles, req.RoleIDs)
		database.DB.Model(&user).Association("Roles").Replace(roles)
	} else {
		database.DB.Model(&user).Association("Roles").Clear()
	}
	_ = casbin.SyncFromDB()
	utils.SuccessMsg(c, "更新成功")
}

// DeleteUser 删除用户
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.User{}, id)
	_ = casbin.SyncFromDB()
	utils.SuccessMsg(c, "删除成功")
}

// GetUser 用户详情
func GetUser(c *gin.Context) {
	id := c.Param("id")
	var user model.User
	if err := database.DB.Preload("Roles").First(&user, id).Error; err != nil {
		utils.Fail(c, "用户不存在")
		return
	}
	utils.Success(c, user)
}
