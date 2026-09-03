package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListAuditLogs 操作日志列表
func ListAuditLogs(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	module := c.Query("module")
	q := database.DB.Model(&model.AuditLog{})
	if keyword != "" {
		q = q.Where("username LIKE ? OR action LIKE ? OR detail LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if module != "" {
		q = q.Where("module = ?", module)
	}
	var total int64
	q.Count(&total)
	var list []model.AuditLog
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}
