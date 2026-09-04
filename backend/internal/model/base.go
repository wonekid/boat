package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 公共字段
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// User 用户
type User struct {
	BaseModel
	Username    string     `json:"username" gorm:"size:64;uniqueIndex;not null"`
	Password    string     `json:"-" gorm:"size:128;not null"`
	Nickname    string     `json:"nickname" gorm:"size:64"`
	Email       string     `json:"email" gorm:"size:128"`
	Phone       string     `json:"phone" gorm:"size:32"`
	Avatar      string     `json:"avatar" gorm:"size:255"`
	Status      int        `json:"status" gorm:"default:1"` // 1启用 0禁用
	DeptID      uint       `json:"deptId"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	MFAEnabled  bool       `json:"mfaEnabled" gorm:"column:mfa_enabled;default:0"` // 是否开启 TOTP 两步验证
	MFASecret   string     `json:"-" gorm:"column:mfa_secret;size:255"`            // 加密存储的 TOTP 密钥（base32）
	Roles       []*Role    `json:"roles" gorm:"many2many:user_roles;"`
}

// Role 角色
type Role struct {
	BaseModel
	Name   string  `json:"name" gorm:"size:64;not null"`
	Code   string  `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Status int     `json:"status" gorm:"default:1"`
	Remark string  `json:"remark" gorm:"size:255"`
	Menus  []*Menu `json:"menus" gorm:"many2many:role_menus;"`
}

// Menu 菜单/按钮权限
type Menu struct {
	BaseModel
	ParentID   uint   `json:"parentId" gorm:"default:0;index"`
	Name       string `json:"name" gorm:"size:64;not null"` // 显示名称
	Type       int    `json:"type" gorm:"default:1"`        // 1目录 2菜单 3按钮
	Permission string `json:"permission" gorm:"size:128"`   // 权限标识（按钮/接口）
	Path       string `json:"path" gorm:"size:255"`         // 路由/接口路径
	Component  string `json:"component" gorm:"size:255"`    // 前端组件
	Icon       string `json:"icon" gorm:"size:64"`
	Sort       int    `json:"sort" gorm:"default:0"`
	Status     int    `json:"status" gorm:"default:1"`
}

// Dept 部门
type Dept struct {
	BaseModel
	ParentID uint   `json:"parentId" gorm:"default:0;index"`
	Name     string `json:"name" gorm:"size:64;not null"`
	Sort     int    `json:"sort" gorm:"default:0"`
	Status   int    `json:"status" gorm:"default:1"`
	Leader   string `json:"leader" gorm:"size:64"`
	Phone    string `json:"phone" gorm:"size:32"`
}

// Host 主机
type Host struct {
	BaseModel
	Name         string `json:"name" gorm:"size:128;not null"`
	IP           string `json:"ip" gorm:"size:64;not null;index"`
	Port         int    `json:"port" gorm:"default:22"`
	OS           string `json:"os" gorm:"size:32"`
	Status       int    `json:"status" gorm:"default:0"` // 0离线 1在线
	GroupID      uint   `json:"groupId"`
	CredentialID uint   `json:"credentialId"`
	User         string `json:"user" gorm:"size:64"`                            // 连接用户名（覆盖凭证）
	BecomeRoot   bool   `json:"becomeRoot" gorm:"column:become_root;default:0"` // 登录后切换 root（sudo -i），用于禁止 root 直登的主机
	CPU          string `json:"cpu" gorm:"size:64"`
	Mem          string `json:"mem" gorm:"size:64"`
	Disk         string `json:"disk" gorm:"size:64"`
	Remark       string `json:"remark" gorm:"size:255"`
}

// Credential 凭证
type Credential struct {
	BaseModel
	Name         string `json:"name" gorm:"size:128;not null"`
	Type         int    `json:"type" gorm:"default:1"` // 1密码 2密钥
	Username     string `json:"username" gorm:"size:64;not null"`
	AuthPassword string `json:"-" gorm:"size:512"`  // RSA 加密后的密码
	PrivateKey   string `json:"-" gorm:"type:text"` // RSA 加密后的私钥
	Remark       string `json:"remark" gorm:"size:255"`
}

// HostGroup 主机分组
type HostGroup struct {
	BaseModel
	ParentID uint   `json:"parentId" gorm:"default:0;index"`
	Name     string `json:"name" gorm:"size:128;not null"`
	Remark   string `json:"remark" gorm:"size:255"`
}

// Authorization 授权（用户-主机/凭证/组）
type Authorization struct {
	BaseModel
	UserID     uint   `json:"userId" gorm:"index"`
	TargetType string `json:"targetType" gorm:"size:32;index"` // host|credential|hostGroup|credentialGroup
	TargetID   uint   `json:"targetId" gorm:"index"`
}

// Session 会话记录
type Session struct {
	BaseModel
	UserID     uint       `json:"userId" gorm:"index"`
	Username   string     `json:"username" gorm:"size:64"`
	HostID     uint       `json:"hostId" gorm:"index"`
	HostName   string     `json:"hostName" gorm:"size:128"`
	HostIP     string     `json:"hostIp" gorm:"size:64"`
	Protocol   string     `json:"protocol" gorm:"size:16"` // ssh|rdp|vnc
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    *time.Time `json:"endedAt"`
	Duration   int        `json:"duration" gorm:"-"`       // 秒（计算字段）
	Status     int        `json:"status" gorm:"default:1"` // 1进行中 0已结束
	RecordPath string     `json:"recordPath" gorm:"size:255"`
}

// AuditLog 操作/登录日志
type AuditLog struct {
	BaseModel
	UserID   uint   `json:"userId" gorm:"index"`
	Username string `json:"username" gorm:"size:64"`
	Action   string `json:"action" gorm:"size:64"` // 动作
	Module   string `json:"module" gorm:"size:64"` // 模块
	IP       string `json:"ip" gorm:"size:64"`
	Status   int    `json:"status" gorm:"default:1"` // 1成功 0失败
	Detail   string `json:"detail" gorm:"type:text"`
}

// HighRiskCommand 高危指令规则
type HighRiskCommand struct {
	BaseModel
	Name      string `json:"name" gorm:"size:128;not null"`
	Pattern   string `json:"pattern" gorm:"size:255;not null"` // 匹配内容
	MatchType string `json:"matchType" gorm:"size:16"`         // exact|prefix|regex
	RiskLevel string `json:"riskLevel" gorm:"size:16"`         // low|medium|high|critical
	Action    string `json:"action" gorm:"size:16"`            // block|warn
	Enabled   int    `json:"enabled" gorm:"default:1"`
	Builtin   int    `json:"builtin" gorm:"default:0"` // 1内置(不可删)
	Remark    string `json:"remark" gorm:"size:255"`
}

// Script 脚本库
type Script struct {
	BaseModel
	Name    string `json:"name" gorm:"size:128;not null"`
	Lang    string `json:"lang" gorm:"size:16"` // shell|python
	Content string `json:"content" gorm:"type:text"`
	Remark  string `json:"remark" gorm:"size:255"`
}

// TaskTemplate 任务模板
type TaskTemplate struct {
	BaseModel
	Name         string `json:"name" gorm:"size:128;not null"`
	Type         string `json:"type" gorm:"size:16"` // command|script|file
	Command      string `json:"command" gorm:"type:text"`
	ScriptID     uint   `json:"scriptId"`
	CredentialID uint   `json:"credentialId"`
	Timeout      int    `json:"timeout" gorm:"default:300"`
	Remark       string `json:"remark" gorm:"size:255"`
}

// TaskExecution 任务执行记录
type TaskExecution struct {
	BaseModel
	TemplateID uint       `json:"templateId"`
	NeedRoot   bool       `json:"needRoot" gorm:"column:need_root;default:0"` // 执行时切换 root（sudo 提权）
	Name       string     `json:"name" gorm:"size:128"`
	Type       string     `json:"type" gorm:"size:16"`
	HostIDs    string     `json:"hostIds" gorm:"type:text"` // JSON 数组
	Status     string     `json:"status" gorm:"size:16"`    // pending|running|success|partial|failed|canceled
	Result     string     `json:"result" gorm:"type:text"`
	CreatedBy  string     `json:"createdBy" gorm:"size:64"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// TaskSchedule 定时任务（Cron 调度）
type TaskSchedule struct {
	BaseModel
	Name       string     `json:"name" gorm:"size:128;not null"`
	TemplateID uint       `json:"templateId"`
	NeedRoot   bool       `json:"needRoot" gorm:"column:need_root;default:0"` // 调度时切换 root（sudo 提权）
	Type       string     `json:"type" gorm:"size:16"`                        // command|script
	Command    string     `json:"command" gorm:"type:text"`
	ScriptID   uint       `json:"scriptId"`
	Cron       string     `json:"cron" gorm:"size:64;not null"` // 标准 5 段 cron 表达式
	HostIDs    string     `json:"hostIds" gorm:"type:text"`     // JSON 数组
	Enabled    int        `json:"enabled" gorm:"default:1"`     // 1启用 0停用
	Status     string     `json:"status" gorm:"size:16"`        // idle|running|error
	LastRunAt  *time.Time `json:"lastRunAt"`
	NextRunAt  *time.Time `json:"nextRunAt"`
	LastResult string     `json:"lastResult" gorm:"type:text"`
	CreatedBy  string     `json:"createdBy" gorm:"size:64"`
	Remark     string     `json:"remark" gorm:"size:255"`
}

// ApprovalTask 操作审批（任务执行需审批时生成，审批通过后再实际执行）
type ApprovalTask struct {
	BaseModel
	RequesterID   uint       `json:"requesterId"`
	RequesterName string     `json:"requesterName" gorm:"size:64"`
	Type          string     `json:"type" gorm:"size:16"` // command|script
	Command       string     `json:"command" gorm:"type:text"`
	ScriptID      uint       `json:"scriptId"`
	HostIDs       string     `json:"hostIds" gorm:"type:text"` // JSON 数组
	Reason        string     `json:"reason" gorm:"size:255"`   // 申请说明
	Status        string     `json:"status" gorm:"size:16"`    // pending|approved|rejected|executed|canceled
	ApproverID    uint       `json:"approverId"`
	ApproverName  string     `json:"approverName" gorm:"size:64"`
	Comment       string     `json:"comment" gorm:"size:255"` // 审批意见
	ExecutionID   uint       `json:"executionId"`
	SubmittedAt   *time.Time `json:"submittedAt"`
	DecidedAt     *time.Time `json:"decidedAt"`
}

// AgentNode OSP 执行机 Agent 节点（执行机主动外连，无需开放入站端口）
type AgentNode struct {
	BaseModel
	Name         string     `json:"name" gorm:"size:128;not null"`
	Hostname     string     `json:"hostname" gorm:"size:128"`
	IP           string     `json:"ip" gorm:"size:64;index"`
	OS           string     `json:"os" gorm:"size:32"`   // linux|windows|darwin
	Arch         string     `json:"arch" gorm:"size:32"` // amd64|arm64...
	Token        string     `json:"token" gorm:"size:128;uniqueIndex;not null"`
	Status       string     `json:"status" gorm:"size:16;default:offline;index"` // online|offline
	Enabled      int        `json:"enabled" gorm:"default:1"`                    // 1启用 0禁用（禁用后拒绝接入）
	Labels       string     `json:"labels" gorm:"size:255"`                      // 标签，逗号分隔（可按环境/业务圈选节点）
	Version      string     `json:"version" gorm:"size:32"`                      // Agent 版本
	RegisteredAt *time.Time `json:"registeredAt"`                                // 首次接入时间
	LastSeenAt   *time.Time `json:"lastSeenAt" gorm:"index"`                     // 最近心跳
	// 实时指标（心跳上报刷新，用于控制台节点监控）
	CPUUsage  float64 `json:"cpuUsage"`  // CPU 使用率 %
	MemUsage  float64 `json:"memUsage"`  // 内存使用率 %
	DiskUsage float64 `json:"diskUsage"` // 根分区使用率 %
	LoadAvg   string  `json:"loadAvg" gorm:"size:64"`
	Uptime    int64   `json:"uptime"`    // 已运行秒数
	MemTotal  uint64  `json:"memTotal"`  // MB
	MemUsed   uint64  `json:"memUsed"`   // MB
	DiskTotal uint64  `json:"diskTotal"` // GB
	DiskUsed  uint64  `json:"diskUsed"`  // GB
	Remark    string  `json:"remark" gorm:"size:255"`
}

// AgentTask OSP 任务（控制台下发给执行机的命令/脚本）
type AgentTask struct {
	BaseModel
	Name       string     `json:"name" gorm:"size:128"`
	Type       string     `json:"type" gorm:"size:16"` // command|script
	Lang       string     `json:"lang" gorm:"size:16"` // shell|python|powershell|batch
	Content    string     `json:"content" gorm:"type:text"`
	ScriptID   uint       `json:"scriptId"`                              // 脚本库来源（可为空）
	Timeout    int        `json:"timeout" gorm:"default:120"`            // 单节点超时秒数
	RunAsUser  string     `json:"runAsUser" gorm:"size:64"`              // 指定执行用户（空=Agent 自身运行用户）
	NodeIDs    string     `json:"nodeIds" gorm:"type:text"`              // JSON 数组
	Status     string     `json:"status" gorm:"size:16;default:running"` // running|success|partial|failed|canceled
	Progress   string     `json:"progress" gorm:"size:32"`               // 完成进度，如 3/5
	CreatedBy  string     `json:"createdBy" gorm:"size:64"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// AgentTaskResult 单节点执行结果
type AgentTaskResult struct {
	BaseModel
	TaskID     uint       `json:"taskId" gorm:"index"`
	NodeID     uint       `json:"nodeId" gorm:"index"`
	NodeName   string     `json:"nodeName" gorm:"size:128"`
	NodeIP     string     `json:"nodeIp" gorm:"size:64"`
	Status     string     `json:"status" gorm:"size:16;default:pending"` // pending|running|success|failed|timeout|offline|canceled
	ExitCode   int        `json:"exitCode"`
	Stdout     string     `json:"stdout" gorm:"type:text"`
	Stderr     string     `json:"stderr" gorm:"type:text"`
	Error      string     `json:"error" gorm:"type:text"`
	Duration   int64      `json:"duration"` // 毫秒
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// 全部可迁移模型
var AllModels = []interface{}{
	&User{}, &Role{}, &Menu{}, &Dept{},
	&Host{}, &Credential{}, &HostGroup{}, &Authorization{},
	&Session{}, &AuditLog{}, &HighRiskCommand{},
	&Script{}, &TaskTemplate{}, &TaskExecution{}, &TaskSchedule{}, &ApprovalTask{},
	&AgentNode{}, &AgentTask{}, &AgentTaskResult{},
}
