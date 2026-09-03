package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/scheduler"
	"boat/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListSchedules 定时任务列表
func ListSchedules(c *gin.Context) {
	var list []model.TaskSchedule
	database.DB.Order("id DESC").Find(&list)
	utils.Success(c, list)
}

// ScheduleReq 定时任务请求
type ScheduleReq struct {
	Name       string `json:"name" binding:"required"`
	Type       string `json:"type"`
	Command    string `json:"command"`
	ScriptID   uint   `json:"scriptId"`
	TemplateID uint   `json:"templateId"`
	NeedRoot   bool   `json:"needRoot"`
	Cron       string `json:"cron" binding:"required"`
	HostIDs    []uint `json:"hostIds"`
	Enabled    int    `json:"enabled"`
	Remark     string `json:"remark"`
}

// CreateSchedule 新建定时任务
func CreateSchedule(c *gin.Context) {
	var req ScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if err := scheduler.ValidateCron(req.Cron); err != nil {
		utils.Fail(c, "Cron 表达式无效: "+err.Error())
		return
	}
	if len(req.HostIDs) == 0 {
		utils.Fail(c, "请选择目标主机")
		return
	}
	_, uname := middlewareUser(c)
	if req.Type == "" {
		req.Type = "command"
	}
	enabled := req.Enabled
	if enabled == 0 {
		enabled = 1
	}
	ts := model.TaskSchedule{
		Name: req.Name, TemplateID: req.TemplateID, NeedRoot: req.NeedRoot, Type: req.Type, Command: req.Command, ScriptID: req.ScriptID,
		Cron: req.Cron, HostIDs: mustJSON(req.HostIDs), Enabled: enabled,
		Remark: req.Remark, CreatedBy: uname, Status: "idle",
	}
	database.DB.Create(&ts)
	if ts.Enabled == 1 {
		scheduler.Register(ts)
	}
	utils.SuccessMsg(c, "创建成功")
}

// UpdateSchedule 更新定时任务
func UpdateSchedule(c *gin.Context) {
	id := c.Param("id")
	var req ScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if err := scheduler.ValidateCron(req.Cron); err != nil {
		utils.Fail(c, "Cron 表达式无效: "+err.Error())
		return
	}
	if len(req.HostIDs) == 0 {
		utils.Fail(c, "请选择目标主机")
		return
	}
	var ts model.TaskSchedule
	if err := database.DB.First(&ts, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	ts.Name = req.Name
	ts.Type = req.Type
	if ts.Type == "" {
		ts.Type = "command"
	}
	ts.Command = req.Command
	ts.TemplateID = req.TemplateID
	ts.NeedRoot = req.NeedRoot
	ts.ScriptID = req.ScriptID
	ts.Cron = req.Cron
	ts.HostIDs = mustJSON(req.HostIDs)
	ts.Remark = req.Remark
	if req.Enabled != 0 {
		ts.Enabled = req.Enabled
	}
	database.DB.Save(&ts)
	scheduler.Reload(ts)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteSchedule 删除定时任务
func DeleteSchedule(c *gin.Context) {
	id := c.Param("id")
	if idd, err := strconv.ParseUint(id, 10, 64); err == nil {
		scheduler.Unregister(uint(idd))
	}
	database.DB.Delete(&model.TaskSchedule{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// ToggleSchedule 启用/停用
func ToggleSchedule(c *gin.Context) {
	id := c.Param("id")
	var ts model.TaskSchedule
	if err := database.DB.First(&ts, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	if ts.Enabled == 1 {
		ts.Enabled = 0
		scheduler.Unregister(ts.ID)
	} else {
		ts.Enabled = 1
		scheduler.Register(ts)
	}
	database.DB.Save(&ts)
	utils.SuccessMsg(c, "操作成功")
}

// RunScheduleNow 立即执行一次
func RunScheduleNow(c *gin.Context) {
	id := c.Param("id")
	var ts model.TaskSchedule
	if err := database.DB.First(&ts, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	execID := scheduler.RunNow(ts)
	utils.Success(c, gin.H{"executionId": execID})
}
