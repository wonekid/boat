package controller

import (
	"boat/internal/casbin"
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// ApprovalReq 提交审批申请（与快速执行同构，增加申请说明）
type ApprovalReq struct {
	Type     string `json:"type" binding:"required"` // command|script
	HostIDs  []uint `json:"hostIds"`
	Command  string `json:"command"`
	ScriptID uint   `json:"scriptId"`
	Reason   string `json:"reason"`
}

// CreateApproval 提交任务执行审批申请（pending 状态，待审批人处理）
func CreateApproval(c *gin.Context) {
	var req ApprovalReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.HostIDs) == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	uid, uname := middlewareUser(c)
	cmd := ResolveExecCommand(req.Type, req.ScriptID, req.Command)
	now := time.Now()
	a := model.ApprovalTask{
		RequesterID:   uid,
		RequesterName: uname,
		Type:          req.Type,
		Command:       cmd,
		ScriptID:      req.ScriptID,
		HostIDs:       mustJSON(req.HostIDs),
		Reason:        req.Reason,
		Status:        "pending",
		SubmittedAt:   &now,
	}
	database.DB.Create(&a)
	writeAudit(uid, uname, "提交执行审批", "任务", c.ClientIP(), 1, "申请ID:"+itoa(int(a.ID)))
	utils.Success(c, gin.H{"approvalId": a.ID})
}

// ListApprovals 审批列表（可按状态、关键字过滤；requester 仅看自己的，审批人看全部 pending）
func ListApprovals(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	status := c.Query("status")
	keyword := c.Query("keyword")
	uid, _ := middlewareUser(c)

	q := database.DB.Model(&model.ApprovalTask{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	// 审批人（具备 task:approval:approve 权限）可见全部；普通用户仅看自己提交的
	if !isApprover(c) {
		q = q.Where("requester_id = ?", uid)
	}
	if keyword != "" {
		q = q.Where("requester_name LIKE ? OR reason LIKE ? OR command LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	q.Count(&total)
	var list []model.ApprovalTask
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}

// GetApproval 审批详情
func GetApproval(c *gin.Context) {
	id := c.Param("id")
	var a model.ApprovalTask
	if err := database.DB.First(&a, id).Error; err != nil {
		utils.Fail(c, "审批单不存在")
		return
	}
	utils.Success(c, a)
}

// requireApprover 校验当前用户是否为审批人（管理员或具备 task:approval:approve 权限）
func requireApprover(c *gin.Context) (uint, string, bool) {
	uid, username := middlewareUser(c)
	if uid == 1 {
		return uid, username, true
	}
	ok, _ := casbin.Check(username, "/task/approval", "task:approval:approve")
	return uid, username, ok
}

func isApprover(c *gin.Context) bool {
	_, _, ok := requireApprover(c)
	return ok
}

// ApproveApproval 审批通过：执行任务并关联交易执行记录
func ApproveApproval(c *gin.Context) {
	uid, uname, ok := requireApprover(c)
	if !ok {
		utils.Forbidden(c, "无审批权限")
		return
	}
	id := c.Param("id")
	var a model.ApprovalTask
	if err := database.DB.First(&a, id).Error; err != nil {
		utils.Fail(c, "审批单不存在")
		return
	}
	if a.Status != "pending" {
		utils.Fail(c, "该申请已处理")
		return
	}
	if a.RequesterID == uid {
		utils.Forbidden(c, "不能审批自己的申请")
		return
	}
	var hostIDs []uint
	_ = jsonUnmarshal(a.HostIDs, &hostIDs)
	execID := LaunchExecution(a.Type, hostIDs, a.Command, a.RequesterID, a.RequesterName, "审批执行-"+itoa(int(a.ID)), 0, 0, false)
	now := time.Now()
	a.Status = "executed"
	a.ApproverID = uid
	a.ApproverName = uname
	a.ExecutionID = execID
	a.DecidedAt = &now
	database.DB.Save(&a)
	writeAudit(uid, uname, "审批通过", "任务", c.ClientIP(), 1, "申请ID:"+itoa(int(a.ID)))
	utils.Success(c, gin.H{"executionId": execID})
}

// RejectApproval 审批拒绝
type RejectReq struct {
	Comment string `json:"comment"`
}

func RejectApproval(c *gin.Context) {
	uid, uname, ok := requireApprover(c)
	if !ok {
		utils.Forbidden(c, "无审批权限")
		return
	}
	id := c.Param("id")
	var a model.ApprovalTask
	if err := database.DB.First(&a, id).Error; err != nil {
		utils.Fail(c, "审批单不存在")
		return
	}
	if a.Status != "pending" {
		utils.Fail(c, "该申请已处理")
		return
	}
	if a.RequesterID == uid {
		utils.Forbidden(c, "不能审批自己的申请")
		return
	}
	var req RejectReq
	_ = c.ShouldBindJSON(&req)
	now := time.Now()
	a.Status = "rejected"
	a.ApproverID = uid
	a.ApproverName = uname
	a.Comment = req.Comment
	a.DecidedAt = &now
	database.DB.Save(&a)
	writeAudit(uid, uname, "审批拒绝", "任务", c.ClientIP(), 1, "申请ID:"+itoa(int(a.ID)))
	utils.SuccessMsg(c, "已拒绝")
}

// CancelApproval 申请人撤回（仅 pending 可撤）
func CancelApproval(c *gin.Context) {
	uid, _ := middlewareUser(c)
	id := c.Param("id")
	var a model.ApprovalTask
	if err := database.DB.First(&a, id).Error; err != nil {
		utils.Fail(c, "审批单不存在")
		return
	}
	if a.RequesterID != uid {
		utils.Forbidden(c, "只能撤回自己的申请")
		return
	}
	if a.Status != "pending" {
		utils.Fail(c, "该申请已处理，无法撤回")
		return
	}
	a.Status = "canceled"
	database.DB.Save(&a)
	utils.SuccessMsg(c, "已撤回")
}
