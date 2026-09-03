package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListDepts 部门树
func ListDepts(c *gin.Context) {
	var list []model.Dept
	database.DB.Order("sort ASC, id ASC").Find(&list)
	utils.Success(c, buildDeptTree(list, 0))
}

// DeptReq 部门请求
type DeptReq struct {
	ParentID uint   `json:"parentId"`
	Name     string `json:"name" binding:"required"`
	Sort     int    `json:"sort"`
	Status   int    `json:"status"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
}

// CreateDept 新建部门
func CreateDept(c *gin.Context) {
	var req DeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	dept := model.Dept{
		ParentID: req.ParentID, Name: req.Name, Sort: req.Sort,
		Status: req.Status, Leader: req.Leader, Phone: req.Phone,
	}
	database.DB.Create(&dept)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateDept 更新部门
func UpdateDept(c *gin.Context) {
	id := c.Param("id")
	var req DeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var dept model.Dept
	if err := database.DB.First(&dept, id).Error; err != nil {
		utils.Fail(c, "部门不存在")
		return
	}
	dept.ParentID = req.ParentID
	dept.Name = req.Name
	dept.Sort = req.Sort
	dept.Status = req.Status
	dept.Leader = req.Leader
	dept.Phone = req.Phone
	database.DB.Save(&dept)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteDept 删除部门
func DeleteDept(c *gin.Context) {
	id := c.Param("id")
	var cnt int64
	database.DB.Model(&model.Dept{}).Where("parent_id = ?", id).Count(&cnt)
	if cnt > 0 {
		utils.Fail(c, "请先删除子部门")
		return
	}
	database.DB.Delete(&model.Dept{}, id)
	utils.SuccessMsg(c, "删除成功")
}

func buildDeptTree(depts []model.Dept, parentID uint) []gin.H {
	var tree []gin.H
	for _, d := range depts {
		if d.ParentID != parentID {
			continue
		}
		children := buildDeptTree(depts, d.ID)
		node := gin.H{
			"id": d.ID, "parentId": d.ParentID, "name": d.Name,
			"sort": d.Sort, "status": d.Status, "leader": d.Leader, "phone": d.Phone,
		}
		if len(children) > 0 {
			node["children"] = children
		}
		tree = append(tree, node)
	}
	return tree
}
