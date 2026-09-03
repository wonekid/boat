package controller

import (
	"boat/internal/config"
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ListSessions 会话列表
func ListSessions(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	q := database.DB.Model(&model.Session{})
	if keyword != "" {
		q = q.Where("host_ip LIKE ? OR username LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	q.Count(&total)
	var list []model.Session
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	for i := range list {
		if list[i].EndedAt != nil {
			list[i].Duration = int(list[i].EndedAt.Sub(list[i].StartedAt).Seconds())
		} else {
			list[i].Duration = int(time.Since(list[i].StartedAt).Seconds())
		}
	}
	utils.Page(c, list, total, page, pageSize)
}

// GetSession 会话详情
func GetSession(c *gin.Context) {
	id := c.Param("id")
	var s model.Session
	if err := database.DB.First(&s, id).Error; err != nil {
		utils.Fail(c, "会话不存在")
		return
	}
	utils.Success(c, s)
}

// TerminateSession 强制结束会话（演示：置状态为已结束）
func TerminateSession(c *gin.Context) {
	id := c.Param("id")
	now := time.Now()
	database.DB.Model(&model.Session{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": 0, "ended_at": &now})
	utils.SuccessMsg(c, "已终止")
}

// GetSessionRecording 获取会话录像文件（asciinema v2 格式），需鉴权
func GetSessionRecording(c *gin.Context) {
	id := c.Param("id")
	var s model.Session
	if err := database.DB.First(&s, id).Error; err != nil {
		utils.Fail(c, "会话不存在")
		return
	}
	if s.RecordPath == "" {
		utils.Fail(c, "该会话无录像记录")
		return
	}
	// 安全校验：仅允许读取录像目录下的文件，防止路径穿越
	base := config.Global.Record.Path
	if !strings.HasPrefix(s.RecordPath, base) {
		utils.Fail(c, "录像路径不合法")
		return
	}
	c.File(s.RecordPath)
}
