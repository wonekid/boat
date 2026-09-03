package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// Dashboard 仪表盘统计
func Dashboard(c *gin.Context) {
	var (
		hostTotal, hostOnline, userTotal, sessionTotal, auditTotal, riskTotal int64
	)
	database.DB.Model(&model.Host{}).Count(&hostTotal)
	database.DB.Model(&model.Host{}).Where("status = ?", 1).Count(&hostOnline)
	database.DB.Model(&model.User{}).Count(&userTotal)
	database.DB.Model(&model.Session{}).Count(&sessionTotal)
	database.DB.Model(&model.AuditLog{}).Count(&auditTotal)
	database.DB.Model(&model.HighRiskCommand{}).Where("enabled = ?", 1).Count(&riskTotal)

	// 近 7 天审计趋势
	type trendItem struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	var trend []trendItem
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var cnt int64
		database.DB.Model(&model.AuditLog{}).
			Where("DATE(created_at) = ?", d).Count(&cnt)
		trend = append(trend, trendItem{Date: d, Count: cnt})
	}

	utils.Success(c, gin.H{
		"stats": gin.H{
			"hostTotal":   hostTotal,
			"hostOnline":  hostOnline,
			"userTotal":   userTotal,
			"sessionTotal": sessionTotal,
			"auditTotal":  auditTotal,
			"riskTotal":   riskTotal,
		},
		"trend": trend,
	})
}
