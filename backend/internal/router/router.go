package router

import (
	"boat/internal/controller"
	"boat/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Register 注册路由
func Register(r *gin.Engine) {
	api := r.Group("/api")

	// 公开
	api.POST("/auth/login", controller.Login)
	api.POST("/auth/mfa/verify", controller.MFAVerify)
	api.GET("/captcha", controller.Captcha)

	// 需鉴权
	auth := api.Group("")
	auth.Use(middleware.JWTAuth())
	{
		auth.GET("/profile", controller.Profile)
		auth.POST("/password", controller.ChangePassword)

		// MFA（两步验证）
		auth.POST("/mfa/setup", controller.MFASetup)
		auth.POST("/mfa/enable", controller.MFAEnable)
		auth.POST("/mfa/disable", controller.MFADisable)

		// 系统管理
		auth.GET("/users", controller.ListUsers)
		auth.GET("/users/:id", controller.GetUser)
		auth.POST("/users", controller.CreateUser)
		auth.PUT("/users/:id", controller.UpdateUser)
		auth.DELETE("/users/:id", controller.DeleteUser)

		auth.GET("/roles", controller.ListRoles)
		auth.GET("/roles/:id", controller.GetRole)
		auth.POST("/roles", controller.CreateRole)
		auth.PUT("/roles/:id", controller.UpdateRole)
		auth.DELETE("/roles/:id", controller.DeleteRole)

		auth.GET("/menus", controller.ListMenus)
		auth.POST("/menus", controller.CreateMenu)
		auth.PUT("/menus/:id", controller.UpdateMenu)
		auth.DELETE("/menus/:id", controller.DeleteMenu)

		auth.GET("/depts", controller.ListDepts)
		auth.POST("/depts", controller.CreateDept)
		auth.PUT("/depts/:id", controller.UpdateDept)
		auth.DELETE("/depts/:id", controller.DeleteDept)

		// 资产
		auth.GET("/hosts", controller.ListHosts)
		auth.GET("/hosts/:id", controller.GetHost)
		auth.POST("/hosts", controller.CreateHost)
		auth.POST("/hosts/batch", controller.BatchCreateHosts)
		auth.PUT("/hosts/:id", controller.UpdateHost)
		auth.DELETE("/hosts/:id", controller.DeleteHost)

		auth.GET("/credentials", controller.ListCredentials)
		auth.POST("/credentials", controller.CreateCredential)
		auth.PUT("/credentials/:id", controller.UpdateCredential)
		auth.DELETE("/credentials/:id", controller.DeleteCredential)
		auth.POST("/credentials/:id/test", controller.TestCredential)

		auth.GET("/host-groups", controller.ListHostGroups)
		auth.POST("/host-groups", controller.CreateHostGroup)
		auth.PUT("/host-groups/:id", controller.UpdateHostGroup)
		auth.DELETE("/host-groups/:id", controller.DeleteHostGroup)

		auth.GET("/auths", controller.ListAuthorizations)
		auth.POST("/auths", controller.CreateAuthorization)
		auth.DELETE("/auths/:id", controller.DeleteAuthorization)

		// 运维
		auth.GET("/sessions", controller.ListSessions)
		auth.GET("/sessions/:id", controller.GetSession)
		auth.GET("/sessions/:id/recording", controller.GetSessionRecording)
		auth.POST("/sessions/:id/terminate", controller.TerminateSession)

		auth.GET("/audits", controller.ListAuditLogs)

		// 安全
		auth.GET("/risks", controller.ListHighRisk)
		auth.POST("/risks", controller.CreateHighRisk)
		auth.PUT("/risks/:id", controller.UpdateHighRisk)
		auth.DELETE("/risks/:id", controller.DeleteHighRisk)

		// 任务
		auth.GET("/scripts", controller.ListScripts)
		auth.GET("/scripts/:id", controller.GetScript)
		auth.POST("/scripts", controller.CreateScript)
		auth.POST("/scripts/upload", controller.UploadScript)
		auth.PUT("/scripts/:id", controller.UpdateScript)
		auth.DELETE("/scripts/:id", controller.DeleteScript)

		auth.GET("/templates", controller.ListTemplates)
		auth.POST("/templates", controller.CreateTemplate)
		auth.PUT("/templates/:id", controller.UpdateTemplate)
		auth.DELETE("/templates/:id", controller.DeleteTemplate)

		auth.GET("/executions", controller.ListExecutions)
		auth.POST("/executions/quick", controller.QuickExecute)

		// 定时任务
		auth.GET("/schedules", controller.ListSchedules)
		auth.POST("/schedules", controller.CreateSchedule)
		auth.PUT("/schedules/:id", controller.UpdateSchedule)
		auth.DELETE("/schedules/:id", controller.DeleteSchedule)
		auth.POST("/schedules/:id/toggle", controller.ToggleSchedule)
		auth.POST("/schedules/:id/run", controller.RunScheduleNow)

		// 操作审批流
		auth.POST("/approvals", controller.CreateApproval)
		auth.GET("/approvals", controller.ListApprovals)
		auth.GET("/approvals/:id", controller.GetApproval)
		auth.POST("/approvals/:id/approve", controller.ApproveApproval)
		auth.POST("/approvals/:id/reject", controller.RejectApproval)
		auth.POST("/approvals/:id/cancel", controller.CancelApproval)

		// 仪表盘
		auth.GET("/dashboard", controller.Dashboard)

		// OSP Agent 执行机管控（自定义端口加密协议通道）
		auth.GET("/agent/overview", controller.AgentOverview)
		auth.GET("/agent/nodes", controller.ListAgentNodes)
		auth.POST("/agent/nodes", controller.CreateAgentNode)
		auth.GET("/agent/nodes/:id", controller.GetAgentNode)
		auth.PUT("/agent/nodes/:id", controller.UpdateAgentNode)
		auth.DELETE("/agent/nodes/:id", controller.DeleteAgentNode)
		auth.POST("/agent/nodes/:id/reset-token", controller.ResetAgentNodeToken)
		auth.POST("/agent/nodes/:id/disconnect", controller.DisconnectAgentNode)
		auth.GET("/agent/nodes/:id/install", controller.GetAgentNodeInstall)
		auth.GET("/agent/scripts", controller.AgentScriptList)
		auth.GET("/agent/tasks", controller.ListAgentTasks)
		auth.POST("/agent/tasks", controller.CreateAgentTask)
		auth.GET("/agent/tasks/:id", controller.GetAgentTask)
		auth.POST("/agent/tasks/:id/cancel", controller.CancelAgentTask)
		auth.POST("/agent/tasks/:id/retry", controller.RetryAgentTaskFailed)
	}

	// WebSocket（令牌经 query 传递，由 handler 内部校验）
	api.GET("/ws/terminal", controller.TerminalWS)
	api.GET("/ws/agent", controller.AgentMonitorWS)
	// Agent 二进制下载（令牌经 query 传递）
	api.GET("/agent/download/:name", controller.DownloadAgent)
}
