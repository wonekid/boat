package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"boat/internal/config"
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/osp"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ============ 执行机节点（OSP Agent） ============

// AgentPublicKey 返回服务端 RSA 公钥的明文 PEM（与握手签名所用私钥成对）。
// 用于执行机侧获取 `server.pem`：直接取「正在运行的服务端」的公钥，
// 规避宿主机与容器内 configs/rsa_key 不一致导致的验签失败。
func AgentPublicKey(c *gin.Context) {
	pem, err := utils.PublicKeyPEM()
	if err != nil {
		utils.Fail(c, "服务端 RSA 公钥未就绪: "+err.Error())
		return
	}
	c.Header("Content-Type", "application/x-pem-file")
	c.Header("Content-Disposition", "inline")
	c.String(http.StatusOK, pem)
}



// ListAgentNodes 节点列表（含实时状态与指标）
func ListAgentNodes(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	status := c.Query("status")
	label := c.Query("label")

	q := database.DB.Model(&model.AgentNode{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR ip LIKE ? OR hostname LIKE ?", like, like, like)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if label != "" {
		q = q.Where("labels LIKE ?", "%"+label+"%")
	}
	var total int64
	q.Count(&total)
	var list []model.AgentNode
	q.Order("status DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	for i := range list {
		if list[i].Status == "" {
			list[i].Status = osp.StatusOffline
		}
	}
	utils.Page(c, list, total, page, pageSize)
}

// GetAgentNode 节点详情
func GetAgentNode(c *gin.Context) {
	id := c.Param("id")
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	utils.Success(c, node)
}

// AgentNodeReq 节点新增/编辑
type AgentNodeReq struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Labels   string `json:"labels"`
	Enabled  *int   `json:"enabled"`
	Remark   string `json:"remark"`
}

// CreateAgentNode 创建节点（自动生成接入令牌，令牌仅在创建时回显）
func CreateAgentNode(c *gin.Context) {
	var req AgentNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.Hostname
	}
	if name == "" {
		name = "agent-" + time.Now().Format("0102150405")
	}
	node := model.AgentNode{
		Name:     name,
		Hostname: req.Hostname,
		IP:       req.IP,
		Labels:   req.Labels,
		Token:    osp.GenerateToken(),
		Status:   osp.StatusOffline,
		Enabled:  1,
		Remark:   req.Remark,
	}
	database.DB.Create(&node)
	uid, uname := middlewareUser(c)
	writeAudit(uid, uname, "创建执行机节点", "Agent管控", clientIP(c), 1, fmt.Sprintf("节点 %s(id=%d)", node.Name, node.ID))
	utils.Success(c, gin.H{"id": node.ID, "token": node.Token})
}

// UpdateAgentNode 更新节点（名称/标签/备注/启停）
func UpdateAgentNode(c *gin.Context) {
	id := c.Param("id")
	var req AgentNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Hostname != "" {
		node.Hostname = req.Hostname
	}
	if req.IP != "" {
		node.IP = req.IP
	}
	if req.Labels != "" {
		node.Labels = req.Labels
	}
	if req.Remark != "" {
		node.Remark = req.Remark
	}
	if req.Enabled != nil {
		node.Enabled = *req.Enabled
		// 禁用时立即断开其在线连接
		if node.Enabled == 0 {
			if s := osp.Default(); s != nil {
				s.Kick(node.ID)
			}
			database.DB.Model(&model.AgentNode{}).Where("id = ?", node.ID).Update("status", osp.StatusOffline)
			node.Status = osp.StatusOffline
		}
	}
	database.DB.Save(&node)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteAgentNode 删除节点及其结果记录
func DeleteAgentNode(c *gin.Context) {
	id := c.Param("id")
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	if s := osp.Default(); s != nil {
		s.Kick(node.ID)
	}
	database.DB.Where("node_id = ?", node.ID).Delete(&model.AgentTaskResult{})
	database.DB.Delete(&model.AgentNode{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// ResetAgentNodeToken 重置接入令牌（旧 Agent 需换令牌重新接入）
func ResetAgentNodeToken(c *gin.Context) {
	id := c.Param("id")
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	node.Token = osp.GenerateToken()
	database.DB.Save(&node)
	if s := osp.Default(); s != nil {
		s.Kick(node.ID)
	}
	utils.Success(c, gin.H{"token": node.Token})
}

// DisconnectAgentNode 强制断开节点连接
func DisconnectAgentNode(c *gin.Context) {
	id := c.Param("id")
	s := osp.Default()
	if s == nil {
		utils.Fail(c, "OSP 服务未启动")
		return
	}
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	if !s.Kick(node.ID) {
		utils.Fail(c, "节点当前不在线")
		return
	}
	utils.SuccessMsg(c, "已断开连接")
}

// GetAgentNodeInstall 生成节点接入指引（含令牌、服务端地址、公钥与一键安装脚本）
func GetAgentNodeInstall(c *gin.Context) {
	id := c.Param("id")
	var node model.AgentNode
	if err := database.DB.First(&node, id).Error; err != nil {
		utils.Fail(c, "节点不存在")
		return
	}
	host := c.Request.Host
	if h := c.GetHeader("X-Forwarded-Host"); h != "" {
		host = h
	}
	if idx := strings.LastIndex(host, ":"); idx > 0 && !strings.Contains(host, "]") {
		host = host[:idx]
	}
	port := config.Global.OSP.Port
	pubKey, err := utils.PublicKeyPEM()
	if err != nil {
		pubKey = ""
	}
	script := fmt.Sprintf(`#!/usr/bin/env bash
# boat osp-agent 一键安装脚本（节点：%s）
set -e
OSP_SERVER="%s:%d"
OSP_TOKEN="%s"
OSP_DIR="/opt/osp-agent"
mkdir -p "$OSP_DIR"
cat > "$OSP_DIR/agent.yaml" <<'EOF'
server: "%s:%d"
token: "%s"
name: "%s"
labels: "%s"
heartbeat: %d
EOF
%scurl -fsSL "http://%s:%d/api/agent/download/osp-agent" -o "$OSP_DIR/osp-agent" || {
  echo "请手动将 osp-agent 二进制放到 $OSP_DIR/ 后重新执行本脚本"; exit 1; }
chmod +x "$OSP_DIR/osp-agent"
cat > /etc/systemd/system/osp-agent.service <<EOF
[Unit]
Description=boat osp-agent
After=network.target

[Service]
Type=simple
WorkingDirectory=$OSP_DIR
ExecStart=$OSP_DIR/osp-agent -c $OSP_DIR/agent.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now osp-agent
systemctl status osp-agent --no-pager | head -20
`, node.Name, host, port, node.Token, host, port, node.Token, node.Name, node.Labels,
		config.Global.OSP.Heartbeat, pubKeyPemBlock(pubKey, node.Name), host, config.Global.Server.Port)

	utils.Success(c, gin.H{
		"nodeId":     node.ID,
		"nodeName":   node.Name,
		"token":      node.Token,
		"serverAddr": fmt.Sprintf("%s:%d", host, port),
		"publicKey":  pubKey,
		"script":     script,
	})
}

// pubKeyPemBlock 生成写入公钥文件的 shell 片段
func pubKeyPemBlock(pubKey, nodeName string) string {
	if strings.TrimSpace(pubKey) == "" {
		return ""
	}
	return fmt.Sprintf("cat > \"/opt/osp-agent/server.pem\" <<'PUBEOF'\n%sPUBEOF\nsed -i 's|^token:|server-pubkey: \"/opt/osp-agent/server.pem\"\\ntoken:|' \"/opt/osp-agent/agent.yaml\"\n", pubKey)
}

// ============ 任务下发 ============

// AgentTaskReq 下发任务请求
type AgentTaskReq struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // command|script
	Lang      string `json:"lang"` // shell|python|powershell|batch
	Content   string `json:"content"`
	ScriptID  uint   `json:"scriptId"`
	Timeout   int    `json:"timeout"`
	RunAsUser string `json:"runAsUser"` // 指定执行用户（如 root / opsuser）
	NodeIDs   []uint `json:"nodeIds"`
}

// CreateAgentTask 下发任务到一批执行机（异步并发，逐节点回传结果）
func CreateAgentTask(c *gin.Context) {
	var req AgentTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	if len(req.NodeIDs) == 0 {
		utils.Fail(c, "请至少选择一个执行机节点")
		return
	}
	taskType := req.Type
	if taskType == "" {
		taskType = "command"
	}
	content := req.Content
	lang := req.Lang
	if taskType == "script" && req.ScriptID > 0 {
		var s model.Script
		if err := database.DB.First(&s, req.ScriptID).Error; err == nil {
			content = s.Content
			if lang == "" {
				lang = s.Lang
			}
		}
	}
	if strings.TrimSpace(content) == "" {
		utils.Fail(c, "任务内容不能为空")
		return
	}
	if lang == "" {
		lang = "shell"
	}
	if req.Timeout <= 0 {
		req.Timeout = 120
	}
	if req.Timeout > 3600 {
		req.Timeout = 3600
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "任务-" + time.Now().Format("0102-150405")
	}

	uid, uname := middlewareUser(c)
	now := time.Now()
	task := model.AgentTask{
		Name:      name,
		Type:      taskType,
		Lang:      lang,
		Content:   content,
		ScriptID:  req.ScriptID,
		Timeout:   req.Timeout,
		RunAsUser: req.RunAsUser,
		NodeIDs:   mustJSON(req.NodeIDs),
		Status:    osp.TaskRunning,
		Progress:  fmt.Sprintf("0/%d", len(req.NodeIDs)),
		CreatedBy: uname,
		StartedAt: &now,
	}
	database.DB.Create(&task)

	// 预建每个节点的结果记录
	for _, nid := range req.NodeIDs {
		var node model.AgentNode
		nodeName, nodeIP := "", ""
		if err := database.DB.First(&node, nid).Error; err == nil {
			nodeName, nodeIP = node.Name, node.IP
		}
		database.DB.Create(&model.AgentTaskResult{
			TaskID:   task.ID,
			NodeID:   nid,
			NodeName: nodeName,
			NodeIP:   nodeIP,
			Status:   osp.ResultPending,
		})
	}

	go runAgentTask(task.ID)
	writeAudit(uid, uname, "下发Agent任务", "Agent管控", clientIP(c), 1,
		fmt.Sprintf("任务 %s(id=%d)，节点数 %d，类型 %s", task.Name, task.ID, len(req.NodeIDs), taskType))
	utils.Success(c, gin.H{"taskId": task.ID})
}

// runAgentTask 并发下发到各节点并汇总状态
func runAgentTask(taskID uint) {
	var task model.AgentTask
	if err := database.DB.First(&task, taskID).Error; err != nil {
		return
	}
	var results []model.AgentTaskResult
	database.DB.Where("task_id = ?", taskID).Find(&results)
	if len(results) == 0 {
		return
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	okCount, failCount := 0, 0

	for i := range results {
		wg.Add(1)
		go func(r *model.AgentTaskResult) {
			defer wg.Done()
			started := time.Now()
			database.DB.Model(&model.AgentTaskResult{}).Where("id = ?", r.ID).
				Updates(map[string]interface{}{"status": osp.ResultRunning, "started_at": started})

			payload := &osp.TaskPayload{
				TaskID:    task.ID,
				ResultID:  r.ID,
				Type:      task.Type,
				Lang:      task.Lang,
				Content:   task.Content,
				Timeout:   task.Timeout,
				RunAsUser: task.RunAsUser,
			}
			res, err := osp.DispatchTask(context.Background(), r.NodeID, payload)
			finished := time.Now()
			updates := map[string]interface{}{"finished_at": finished, "duration": finished.Sub(started).Milliseconds()}
			status := osp.ResultSuccess
			if err != nil {
				status = osp.ResultFailed
				if errors.Is(err, osp.ErrDisconnected) {
					status = osp.ResultOffline
				}
				updates["status"] = status
				updates["error"] = err.Error()
			} else {
				updates["status"] = res.Status
				updates["exit_code"] = res.ExitCode
				updates["stdout"] = truncateOut(res.Stdout)
				updates["stderr"] = truncateOut(res.Stderr)
				updates["error"] = res.Error
				if res.Duration > 0 {
					updates["duration"] = res.Duration
				}
				status = res.Status
			}
			database.DB.Model(&model.AgentTaskResult{}).Where("id = ?", r.ID).Updates(updates)

			mu.Lock()
			if status == osp.ResultSuccess {
				okCount++
			} else {
				failCount++
			}
			database.DB.Model(&model.AgentTask{}).Where("id = ?", taskID).
				Update("progress", fmt.Sprintf("%d/%d", okCount+failCount, len(results)))
			mu.Unlock()
		}(&results[i])
	}
	wg.Wait()

	taskStatus := osp.TaskSuccess
	if failCount == len(results) {
		taskStatus = osp.TaskFailed
	} else if failCount > 0 {
		taskStatus = osp.TaskPartial
	}
	// 期间若被取消则保留取消状态
	var cur model.AgentTask
	if err := database.DB.First(&cur, taskID).Error; err == nil && cur.Status == osp.TaskCanceled {
		return
	}
	now := time.Now()
	database.DB.Model(&model.AgentTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":      taskStatus,
		"finished_at": now,
		"progress":    fmt.Sprintf("%d/%d", len(results), len(results)),
	})
}

// truncateOut 限制回传输出长度，避免超长输出打爆数据库
func truncateOut(s string) string {
	const limit = 512 * 1024
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...[输出过长已截断]"
}

// ListAgentTasks 任务列表
func ListAgentTasks(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	status := c.Query("status")
	q := database.DB.Model(&model.AgentTask{})
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var list []model.AgentTask
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}

// GetAgentTask 任务详情（含逐节点结果）
func GetAgentTask(c *gin.Context) {
	id := c.Param("id")
	var task model.AgentTask
	if err := database.DB.First(&task, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	var results []model.AgentTaskResult
	database.DB.Where("task_id = ?", task.ID).Order("id ASC").Find(&results)
	utils.Success(c, gin.H{"task": task, "results": results})
}

// CancelAgentTask 取消任务（通知在线节点中止，未完成的置为已取消）
func CancelAgentTask(c *gin.Context) {
	id := c.Param("id")
	var task model.AgentTask
	if err := database.DB.First(&task, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	if task.Status != osp.TaskRunning {
		utils.Fail(c, "任务已结束，无需取消")
		return
	}
	var results []model.AgentTaskResult
	database.DB.Where("task_id = ? AND status IN ?", task.ID,
		[]string{osp.ResultPending, osp.ResultRunning}).Find(&results)
	for _, r := range results {
		_ = osp.CancelNodeTask(r.NodeID, r.ID)
		database.DB.Model(&model.AgentTaskResult{}).Where("id = ? AND status IN ?", r.ID,
			[]string{osp.ResultPending, osp.ResultRunning}).
			Updates(map[string]interface{}{"status": osp.ResultCanceled, "finished_at": time.Now()})
	}
	now := time.Now()
	database.DB.Model(&model.AgentTask{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"status": osp.TaskCanceled, "finished_at": now})
	utils.SuccessMsg(c, "已下发取消指令")
}

// RetryAgentTaskFailed 重试失败/离线节点
func RetryAgentTaskFailed(c *gin.Context) {
	id := c.Param("id")
	var task model.AgentTask
	if err := database.DB.First(&task, id).Error; err != nil {
		utils.Fail(c, "任务不存在")
		return
	}
	var results []model.AgentTaskResult
	database.DB.Where("task_id = ? AND status IN ?", task.ID,
		[]string{osp.ResultFailed, osp.ResultOffline, osp.ResultTimeout, osp.ResultCanceled}).Find(&results)
	if len(results) == 0 {
		utils.Fail(c, "没有可重试的节点")
		return
	}
	now := time.Now()
	database.DB.Model(&model.AgentTask{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"status": osp.TaskRunning, "finished_at": nil, "started_at": now})
	for _, r := range results {
		database.DB.Model(&model.AgentTaskResult{}).Where("id = ?", r.ID).
			Updates(map[string]interface{}{"status": osp.ResultPending, "started_at": nil,
				"finished_at": nil, "error": "", "stdout": "", "stderr": "", "exit_code": 0, "duration": 0})
	}
	go runAgentTaskByIDs(task.ID, resultIDs(results))
	utils.Success(c, gin.H{"count": len(results)})
}

func resultIDs(list []model.AgentTaskResult) []uint {
	ids := make([]uint, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.ID)
	}
	return ids
}

// runAgentTaskByIDs 按指定结果记录重试
func runAgentTaskByIDs(taskID uint, ids []uint) {
	var task model.AgentTask
	if err := database.DB.First(&task, taskID).Error; err != nil {
		return
	}
	var results []model.AgentTaskResult
	database.DB.Where("task_id = ? AND id IN ?", taskID, ids).Find(&results)
	if len(results) == 0 {
		return
	}
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(r *model.AgentTaskResult) {
			defer wg.Done()
			started := time.Now()
			database.DB.Model(&model.AgentTaskResult{}).Where("id = ?", r.ID).
				Updates(map[string]interface{}{"status": osp.ResultRunning, "started_at": started})
			res, err := osp.DispatchTask(context.Background(), r.NodeID, &osp.TaskPayload{
				TaskID:    task.ID,
				ResultID:  r.ID,
				Type:      task.Type,
				Lang:      task.Lang,
				Content:   task.Content,
				Timeout:   task.Timeout,
				RunAsUser: task.RunAsUser,
			})
			finished := time.Now()
			updates := map[string]interface{}{"finished_at": finished, "duration": finished.Sub(started).Milliseconds()}
			if err != nil {
				status := osp.ResultFailed
				if errors.Is(err, osp.ErrDisconnected) {
					status = osp.ResultOffline
				}
				updates["status"] = status
				updates["error"] = err.Error()
			} else {
				updates["status"] = res.Status
				updates["exit_code"] = res.ExitCode
				updates["stdout"] = truncateOut(res.Stdout)
				updates["stderr"] = truncateOut(res.Stderr)
				updates["error"] = res.Error
			}
			database.DB.Model(&model.AgentTaskResult{}).Where("id = ?", r.ID).Updates(updates)
		}(&results[i])
	}
	wg.Wait()

	var all []model.AgentTaskResult
	database.DB.Where("task_id = ?", taskID).Find(&all)
	failed := 0
	for _, r := range all {
		if r.Status != osp.ResultSuccess {
			failed++
		}
	}
	status := osp.TaskSuccess
	if failed == len(all) {
		status = osp.TaskFailed
	} else if failed > 0 {
		status = osp.TaskPartial
	}
	now := time.Now()
	database.DB.Model(&model.AgentTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": status, "finished_at": now, "progress": fmt.Sprintf("%d/%d", len(all), len(all)),
	})
}

// ============ 实时监控 ============

// agentWSMessage 实时推送消息
type agentWSMessage struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Node      *osp.NodeStatus        `json:"node,omitempty"`
	Nodes     []*osp.NodeStatus      `json:"nodes,omitempty"`
	Result    *osp.TaskResultPayload `json:"result,omitempty"`
}

// AgentMonitorWS 节点状态实时监控（首帧全量快照 + 后续增量事件）
func AgentMonitorWS(c *gin.Context) {
	token := c.Query("token")
	if _, err := utils.ParseToken(token); err != nil {
		utils.Unauthorized(c, "令牌无效或已过期")
		return
	}
	s := osp.Default()
	if s == nil {
		utils.Fail(c, "OSP 服务未启动")
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[agent-ws] 升级失败: %v", err)
		return
	}
	defer conn.Close()

	sub := s.Hub().Subscribe()
	defer s.Hub().Unsubscribe(sub)

	// 首帧全量快照
	_ = conn.WriteJSON(agentWSMessage{Type: "snapshot", Timestamp: time.Now().UnixMilli(), Nodes: osp.Snapshot()})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	writeMu := sync.Mutex{}
	write := func(msg agentWSMessage) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_ = conn.WriteJSON(msg)
	}

	for {
		select {
		case e, ok := <-sub.Events():
			if !ok {
				return
			}
			write(agentWSMessage{Type: e.Type, Timestamp: e.Timestamp, Node: e.Node, Result: e.Result})
		case <-ticker.C:
			writeMu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_ = conn.WriteMessage(websocket.PingMessage, nil)
			writeMu.Unlock()
		case <-done:
			return
		}
	}
}

// AgentOverview 总览统计（在线节点、任务概况、服务端运行状态）
func AgentOverview(c *gin.Context) {
	online, offline := osp.OnlineStats()
	var running int64
	database.DB.Model(&model.AgentTask{}).Where("status = ?", osp.TaskRunning).Count(&running)
	var today int64
	todayStart := time.Now().Truncate(24 * time.Hour).Add(-8 * time.Hour)
	database.DB.Model(&model.AgentTask{}).Where("created_at >= ?", todayStart).Count(&today)

	serving := false
	port := config.Global.OSP.Port
	if s := osp.Default(); s != nil {
		serving = true
	}
	utils.Success(c, gin.H{
		"online":      online,
		"offline":     offline,
		"runningTask": running,
		"todayTask":   today,
		"port":        port,
		"serving":     serving,
		"heartbeat":   config.Global.OSP.Heartbeat,
	})
}

// DownloadAgent 下载 osp-agent 二进制（令牌经 query 传递，若未构建则返回构建指引）
func DownloadAgent(c *gin.Context) {
	if _, err := utils.ParseToken(c.Query("token")); err != nil {
		utils.Unauthorized(c, "令牌无效或已过期")
		return
	}
	name := c.Param("name")
	if name != "osp-agent" && name != "osp-agent.exe" {
		utils.FailCode(c, http.StatusNotFound, "文件不存在")
		return
	}
	path := agentBinaryPath(name)
	if path == "" {
		utils.FailCode(c, http.StatusNotFound, "服务端尚未构建 osp-agent，请在 backend 目录执行: go build -o dist/osp-agent ./cmd/agent")
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.File(path)
}

// agentBinaryPath 定位已构建的 agent 二进制
func agentBinaryPath(name string) string {
	candidates := []string{
		"dist/" + name,
		"../backend/dist/" + name,
		"../dist/" + name,
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// AgentScriptList 供前端选择脚本库脚本（复用脚本管理能力）
func AgentScriptList(c *gin.Context) {
	var list []model.Script
	database.DB.Order("id DESC").Find(&list)
	utils.Success(c, list)
}
