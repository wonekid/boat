package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListMenus 菜单树（含按钮）
func ListMenus(c *gin.Context) {
	var list []model.Menu
	database.DB.Order("sort ASC, id ASC").Find(&list)
	utils.Success(c, buildMenuTree(list, 0))
}

// MenuReq 菜单请求
type MenuReq struct {
	ParentID  uint   `json:"parentId"`
	Name      string `json:"name" binding:"required"`
	Type      int    `json:"type"`
	Permission string `json:"permission"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	Sort      int    `json:"sort"`
	Status    int    `json:"status"`
}

// CreateMenu 新建菜单
func CreateMenu(c *gin.Context) {
	var req MenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	menu := model.Menu{
		ParentID: req.ParentID, Name: req.Name, Type: req.Type,
		Permission: req.Permission, Path: req.Path, Component: req.Component,
		Icon: req.Icon, Sort: req.Sort, Status: req.Status,
	}
	database.DB.Create(&menu)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateMenu 更新菜单
func UpdateMenu(c *gin.Context) {
	id := c.Param("id")
	var req MenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var menu model.Menu
	if err := database.DB.First(&menu, id).Error; err != nil {
		utils.Fail(c, "菜单不存在")
		return
	}
	menu.ParentID = req.ParentID
	menu.Name = req.Name
	menu.Type = req.Type
	menu.Permission = req.Permission
	menu.Path = req.Path
	menu.Component = req.Component
	menu.Icon = req.Icon
	menu.Sort = req.Sort
	menu.Status = req.Status
	database.DB.Save(&menu)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteMenu 删除菜单
func DeleteMenu(c *gin.Context) {
	id := c.Param("id")
	// 存在子节点则拒绝
	var cnt int64
	database.DB.Model(&model.Menu{}).Where("parent_id = ?", id).Count(&cnt)
	if cnt > 0 {
		utils.Fail(c, "请先删除子菜单")
		return
	}
	database.DB.Delete(&model.Menu{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// buildMenuTree 组装树
func buildMenuTree(menus []model.Menu, parentID uint) []gin.H {
	var tree []gin.H
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}
		children := buildMenuTree(menus, m.ID)
		node := gin.H{
			"id": m.ID, "parentId": m.ParentID, "name": m.Name,
			"type": m.Type, "permission": m.Permission, "path": m.Path,
			"component": m.Component, "icon": m.Icon, "sort": m.Sort,
			"status": m.Status,
		}
		if len(children) > 0 {
			node["children"] = children
		}
		tree = append(tree, node)
	}
	return tree
}
