package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/ssh"
	"boat/internal/utils"
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- 任务模板 ----------

// ListTemplates 模板列表
func ListTemplates(c *gin.Context) {
	var list []model.TaskTemplate
	database.DB.Order("id DESC").Find(&list)
	utils.Success(c, list)
}

// TemplateReq 模板请求
type TemplateReq struct {
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type"`
	Command      string `json:"command"`
	ScriptID     uint   `json:"scriptId"`
	CredentialID uint   `json:"credentialId"`
	Timeout      int    `json:"timeout"`
	Remark       string `json:"remark"`
}

// CreateTemplate 新建模板
func CreateTemplate(c *gin.Context) {
	var req TemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	t := model.TaskTemplate{
		Name: req.Name, Type: req.Type, Command: req.Command,
		ScriptID: req.ScriptID, CredentialID: req.CredentialID,
		Timeout: req.Timeout, Remark: req.Remark,
	}
	if t.Timeout == 0 {
		t.Timeout = 300
	}
	database.DB.Create(&t)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateTemplate 更新模板
func UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req TemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var t model.TaskTemplate
	if err := database.DB.First(&t, id).Error; err != nil {
		utils.Fail(c, "模板不存在")
		return
	}
	t.Name = req.Name
	t.Type = req.Type
	t.Command = req.Command
	t.ScriptID = req.ScriptID
	t.CredentialID = req.CredentialID
	t.Timeout = req.Timeout
	t.Remark = req.Remark
	database.DB.Save(&t)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteTemplate 删除模板
func DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.TaskTemplate{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// ---------- 任务执行 ----------

// ListExecutions 执行记录
func ListExecutions(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	q := database.DB.Model(&model.TaskExecution{})
	var total int64
	q.Count(&total)
	var list []model.TaskExecution
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}

// QuickExecReq 快速执行
type QuickExecReq struct {
	Type       string `json:"type" binding:"required"` // command|script|file
	HostIDs    []uint `json:"hostIds"`
	Command    string `json:"command"`
	ScriptID   uint   `json:"scriptId"`
	TemplateID uint   `json:"templateId"`
	NeedRoot   bool   `json:"needRoot"`
}

// QuickExecute 快速命令/脚本执行（并发，逐主机 SSH）
func QuickExecute(c *gin.Context) {
	var req QuickExecReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.HostIDs) == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	execType := req.Type
	command := req.Command
	scriptID := req.ScriptID
	var templateID, credentialID uint
	if req.TemplateID > 0 {
		var tpl model.TaskTemplate
		if err := database.DB.First(&tpl, req.TemplateID).Error; err == nil {
			templateID = tpl.ID
			credentialID = tpl.CredentialID
			// 以模板为权威配置填充（请求未显式覆盖时）
			if execType == "" {
				execType = tpl.Type
			}
			if command == "" {
				command = tpl.Command
			}
			if scriptID == 0 {
				scriptID = tpl.ScriptID
			}
		}
	}
	if execType == "" {
		execType = "command"
	}
	cmd := ResolveExecCommand(execType, scriptID, command)
	uid, uname := middlewareUser(c)
	name := "快速执行-" + time.Now().Format("150405")
	id := LaunchExecution(execType, req.HostIDs, cmd, uid, uname, name, templateID, credentialID, req.NeedRoot)
	utils.Success(c, gin.H{"executionId": id})
}

// ResolveExecCommand 根据执行类型解析最终下发命令（脚本类型展开脚本内容）
func ResolveExecCommand(execType string, scriptID uint, command string) string {
	cmd := command
	if execType == "script" && scriptID > 0 {
		var s model.Script
		if err := database.DB.First(&s, scriptID).Error; err == nil {
			if s.Lang == "python" {
				cmd = "python3 - <<'BOATEOF'\n" + s.Content + "\nBOATEOF"
			} else {
				cmd = s.Content
			}
		}
	}
	return cmd
}

// LaunchExecution 创建执行记录并异步执行（供快速执行与定时调度共用）
func LaunchExecution(execType string, hostIDs []uint, cmd string, uid uint, uname string, name string, templateID uint, credentialID uint, needRoot bool) uint {
	exec := model.TaskExecution{
		Type: execType, HostIDs: mustJSON(hostIDs), Status: "running",
		Name: name, CreatedBy: uname, TemplateID: templateID,
		StartedAt: ptrTime(time.Now()),
	}
	database.DB.Create(&exec)
	go runOnHosts(exec.ID, hostIDs, cmd, uid, uname, credentialID, needRoot)
	return exec.ID
}

func runOnHosts(execID uint, hostIDs []uint, cmd string, uid uint, uname string, credentialID uint, needRoot bool) {
	type hostResult struct {
		HostID uint   `json:"hostId"`
		IP     string `json:"ip"`
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	var (
		mu      sync.Mutex
		results []hostResult
		wg      sync.WaitGroup
	)
	for _, hid := range hostIDs {
		wg.Add(1)
		go func(hid uint) {
			defer wg.Done()
			var host model.Host
			r := hostResult{HostID: hid}
			if err := database.DB.First(&host, hid).Error; err != nil {
				r.Error = "主机不存在"
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return
			}
			r.IP = host.IP
			if credentialID > 0 {
				host.CredentialID = credentialID
			}
			if needRoot {
				host.BecomeRoot = true
			}
			client, err := ssh.ConnectToHost(host, uid)
			if err != nil {
				r.Error = err.Error()
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return
			}
			defer client.Close()
			out, err := ssh.ExecCmd(client, host, uid, cmd)
			r.Output = out
			if err != nil {
				r.Error = err.Error()
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(hid)
	}
	wg.Wait()

	// 汇总
	var failed int
	for _, r := range results {
		if r.Error != "" {
			failed++
		}
	}
	status := "success"
	if failed == len(results) {
		status = "failed"
	} else if failed > 0 {
		status = "partial"
	}
	now := time.Now()
	database.DB.Model(&model.TaskExecution{}).Where("id = ?", execID).Updates(map[string]interface{}{
		"status":      status,
		"result":      mustJSON(results),
		"finished_at": &now,
	})
	writeAudit(uid, uname, "任务执行完成", "任务编排", "internal", 1, string(mustJSON(results)))
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func ptrTime(t time.Time) *time.Time { return &t }
