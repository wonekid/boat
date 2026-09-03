package controller

import (
	"boat/internal/database"
	"boat/internal/casbin"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListRoles 角色列表
func ListRoles(c *gin.Context) {
	var list []model.Role
	database.DB.Preload("Menus").Order("id").Find(&list)
	utils.Success(c, list)
}

// RoleReq 角色请求
type RoleReq struct {
	Name   string `json:"name" binding:"required"`
	Code   string `json:"code" binding:"required"`
	Status int    `json:"status"`
	Remark string `json:"remark"`
	MenuIDs []uint `json:"menuIds"`
}

// CreateRole 新建角色
func CreateRole(c *gin.Context) {
	var req RoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	role := model.Role{Name: req.Name, Code: req.Code, Status: req.Status, Remark: req.Remark}
	database.DB.Create(&role)
	if len(req.MenuIDs) > 0 {
		var menus []model.Menu
		database.DB.Find(&menus, req.MenuIDs)
		database.DB.Model(&role).Association("Menus").Replace(menus)
	}
	_ = casbin.SyncFromDB()
	utils.SuccessMsg(c, "创建成功")
}

// UpdateRole 更新角色
func UpdateRole(c *gin.Context) {
	id := c.Param("id")
	var req RoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var role model.Role
	if err := database.DB.First(&role, id).Error; err != nil {
		utils.Fail(c, "角色不存在")
		return
	}
	role.Name = req.Name
	role.Status = req.Status
	role.Remark = req.Remark
	database.DB.Save(&role)
	if req.MenuIDs != nil {
		var menus []model.Menu
		if len(req.MenuIDs) > 0 {
			database.DB.Find(&menus, req.MenuIDs)
		}
		database.DB.Model(&role).Association("Menus").Replace(menus)
	}
	_ = casbin.SyncFromDB()
	utils.SuccessMsg(c, "更新成功")
}

// DeleteRole 删除角色
func DeleteRole(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.Role{}, id)
	_ = casbin.SyncFromDB()
	utils.SuccessMsg(c, "删除成功")
}

// GetRole 角色详情
func GetRole(c *gin.Context) {
	id := c.Param("id")
	var role model.Role
	if err := database.DB.Preload("Menus").First(&role, id).Error; err != nil {
		utils.Fail(c, "角色不存在")
		return
	}
	utils.Success(c, role)
}
