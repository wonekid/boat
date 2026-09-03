package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListHostGroups 主机分组树
func ListHostGroups(c *gin.Context) {
	var list []model.HostGroup
	database.DB.Order("id").Find(&list)
	utils.Success(c, buildHostGroupTree(list, 0))
}

// HostGroupReq 分组请求
type HostGroupReq struct {
	ParentID uint   `json:"parentId"`
	Name     string `json:"name" binding:"required"`
	Remark   string `json:"remark"`
}

// CreateHostGroup 新建分组
func CreateHostGroup(c *gin.Context) {
	var req HostGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	g := model.HostGroup{ParentID: req.ParentID, Name: req.Name, Remark: req.Remark}
	database.DB.Create(&g)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateHostGroup 更新分组
func UpdateHostGroup(c *gin.Context) {
	id := c.Param("id")
	var req HostGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var g model.HostGroup
	if err := database.DB.First(&g, id).Error; err != nil {
		utils.Fail(c, "分组不存在")
		return
	}
	g.ParentID = req.ParentID
	g.Name = req.Name
	g.Remark = req.Remark
	database.DB.Save(&g)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteHostGroup 删除分组
func DeleteHostGroup(c *gin.Context) {
	id := c.Param("id")
	var cnt int64
	database.DB.Model(&model.HostGroup{}).Where("parent_id = ?", id).Count(&cnt)
	if cnt > 0 {
		utils.Fail(c, "请先删除子分组")
		return
	}
	database.DB.Delete(&model.HostGroup{}, id)
	utils.SuccessMsg(c, "删除成功")
}

func buildHostGroupTree(groups []model.HostGroup, parentID uint) []gin.H {
	var tree []gin.H
	for _, g := range groups {
		if g.ParentID != parentID {
			continue
		}
		children := buildHostGroupTree(groups, g.ID)
		node := gin.H{"id": g.ID, "parentId": g.ParentID, "name": g.Name, "remark": g.Remark}
		if len(children) > 0 {
			node["children"] = children
		}
		tree = append(tree, node)
	}
	return tree
}
