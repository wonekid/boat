package service

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"log"
)

// Seed 初始化内置数据：管理员、角色、菜单、高危规则
func Seed() error {
	// 管理员
	var adminCnt int64
	database.DB.Model(&model.User{}).Count(&adminCnt)
	if adminCnt == 0 {
		admin := model.User{
			Username: "admin",
			Password: utils.PasswordHash("admin123"),
			Nickname: "超级管理员",
			Status:   1,
		}
		database.DB.Create(&admin)
		log.Println("[seed] 已创建默认管理员 admin / admin123")
	}

	// 角色
	var roleCnt int64
	database.DB.Model(&model.Role{}).Count(&roleCnt)
	if roleCnt == 0 {
		roles := []model.Role{
			{Name: "超级管理员", Code: "superadmin", Status: 1, Remark: "系统内置"},
			{Name: "运维工程师", Code: "ops", Status: 1, Remark: "日常运维"},
			{Name: "审计员", Code: "auditor", Status: 1, Remark: "只读审计"},
		}
		database.DB.CreateInBatches(roles, 10)
		// 管理员绑定 superadmin
		var admin model.User
		var super model.Role
		database.DB.Where("username = ?", "admin").First(&admin)
		database.DB.Where("code = ?", "superadmin").First(&super)
		database.DB.Model(&admin).Association("Roles").Append(&super)
		log.Println("[seed] 已创建默认角色")
	}

	// 菜单（同时作为权限点）
	seedMenus()
	seedScheduleMenu()
	seedApprovalMenu()
	seedAgentMenu()
	fixMenuParents()

	// 高危指令内置规则
	seedHighRisk()

	return nil
}

// fixMenuParents 修正菜单父子关系（按名称校准 parent_id，幂等；
// 种子数据早期 ParentID 引用与插入顺序不一致，导致子菜单错挂，此处统一校正）
func fixMenuParents() {
	expect := map[string]string{
		"用户管理": "系统管理", "角色管理": "系统管理", "菜单管理": "系统管理", "部门管理": "系统管理",
		"主机管理": "资产管理", "凭证管理": "资产管理", "主机分组": "资产管理", "授权管理": "资产管理",
		"Web终端": "运维中心", "会话审计": "运维中心", "操作日志": "运维中心",
		"高危指令": "安全中心",
		"脚本库":  "任务编排", "任务模板": "任务编排", "任务执行": "任务编排", "定时任务": "任务编排", "操作审批": "任务编排",
		"执行机节点": "Agent管控", "Agent任务下发": "Agent管控",
	}
	var all []model.Menu
	database.DB.Find(&all)
	byName := map[string]uint{}
	for _, m := range all {
		byName[m.Name] = m.ID
	}
	for child, parent := range expect {
		cid, ok1 := byName[child]
		pid, ok2 := byName[parent]
		if ok1 && ok2 && cid != pid {
			database.DB.Model(&model.Menu{}).Where("id = ?", cid).Update("parent_id", pid)
		}
	}
	log.Println("[seed] 已校正菜单父子关系")
}

// seedScheduleMenu 幂等补充「定时任务」菜单与权限点（挂任务编排下）
func seedScheduleMenu() {
	var m model.Menu
	database.DB.Where("path = ?", "/task/schedule").FirstOrCreate(&m, model.Menu{
		ParentID:   21, // 初值；fixMenuParents 会按名称校正为「任务编排」
		Name:       "定时任务",
		Type:       2,
		Permission: "task:schedule:list",
		Path:       "/task/schedule",
		Component:  "views/task/schedule",
		Icon:       "Timer",
		Sort:       4,
		Status:     1,
	})
	// 授予超级管理员
	var super model.Role
	if err := database.DB.Where("code = ?", "superadmin").First(&super).Error; err == nil {
		var all []model.Menu
		database.DB.Find(&all)
		database.DB.Model(&super).Association("Menus").Replace(all)
	}
	log.Println("[seed] 已确保定时任务菜单存在")
}

// seedApprovalMenu 幂等补充「操作审批」菜单与权限点（挂任务编排下，
// 作为高危/批量操作的二级审批闸门：提交→审批人通过→自动执行并关联交易记录）
func seedApprovalMenu() {
	var m model.Menu
	database.DB.Where("path = ?", "/task/approval").FirstOrCreate(&m, model.Menu{
		ParentID:   21, // 初值；fixMenuParents 会按名称校正为「任务编排」
		Name:       "操作审批",
		Type:       2,
		Permission: "task:approval:approve",
		Path:       "/task/approval",
		Component:  "views/task/approval",
		Icon:       "Stamp",
		Sort:       5,
		Status:     1,
	})
	// 授予超级管理员
	var super model.Role
	if err := database.DB.Where("code = ?", "superadmin").First(&super).Error; err == nil {
		var all []model.Menu
		database.DB.Find(&all)
		database.DB.Model(&super).Association("Menus").Replace(all)
	}
	log.Println("[seed] 已确保操作审批菜单存在")
}

// seedAgentMenu 幂等补充「Agent管控」目录与子菜单权限点
// （OSP 执行机管控：自定义端口加密协议，用于下发命令/脚本任务与节点实时监控）
func seedAgentMenu() {
	var dir model.Menu
	database.DB.Where("path = ?", "/agent").FirstOrCreate(&dir, model.Menu{
		ParentID: 0,
		Name:     "Agent管控",
		Type:     1,
		Path:     "/agent",
		Icon:     "Platform",
		Sort:     7,
		Status:   1,
	})
	var node model.Menu
	database.DB.Where("path = ?", "/agent/node").FirstOrCreate(&node, model.Menu{
		ParentID:   dir.ID,
		Name:       "执行机节点",
		Type:       2,
		Permission: "agent:node:list",
		Path:       "/agent/node",
		Component:  "views/agent/node",
		Icon:       "Monitor",
		Sort:       1,
		Status:     1,
	})
	var task model.Menu
	database.DB.Where("path = ?", "/agent/task").FirstOrCreate(&task, model.Menu{
		ParentID:   dir.ID,
		Name:       "Agent任务下发",
		Type:       2,
		Permission: "agent:task:list",
		Path:       "/agent/task",
		Component:  "views/agent/task",
		Icon:       "Promotion",
		Sort:       2,
		Status:     1,
	})
	grantMenusToSuper()
	log.Println("[seed] 已确保 Agent管控 菜单存在")
}

// grantMenusToSuper 将全部菜单授权给超级管理员角色
func grantMenusToSuper() {
	var super model.Role
	if err := database.DB.Where("code = ?", "superadmin").First(&super).Error; err == nil {
		var all []model.Menu
		database.DB.Find(&all)
		database.DB.Model(&super).Association("Menus").Replace(all)
	}
}

func seedMenus() {
	var cnt int64
	database.DB.Model(&model.Menu{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	menus := []model.Menu{
		{ParentID: 0, Name: "仪表盘", Type: 2, Permission: "dashboard:view", Path: "/dashboard", Component: "views/dashboard/index", Icon: "Odometer", Sort: 1, Status: 1},

		{ParentID: 0, Name: "系统管理", Type: 1, Path: "/system", Icon: "Setting", Sort: 2, Status: 1},
		{ParentID: 2, Name: "用户管理", Type: 2, Permission: "system:user:list", Path: "/system/user", Component: "views/system/user", Icon: "User", Sort: 1, Status: 1},
		{ParentID: 2, Name: "角色管理", Type: 2, Permission: "system:role:list", Path: "/system/role", Component: "views/system/role", Icon: "Avatar", Sort: 2, Status: 1},
		{ParentID: 2, Name: "菜单管理", Type: 2, Permission: "system:menu:list", Path: "/system/menu", Component: "views/system/menu", Icon: "Menu", Sort: 3, Status: 1},
		{ParentID: 2, Name: "部门管理", Type: 2, Permission: "system:dept:list", Path: "/system/dept", Component: "views/system/dept", Icon: "OfficeBuilding", Sort: 4, Status: 1},

		{ParentID: 0, Name: "资产管理", Type: 1, Path: "/asset", Icon: "Monitor", Sort: 3, Status: 1},
		{ParentID: 8, Name: "主机管理", Type: 2, Permission: "asset:host:list", Path: "/asset/host", Component: "views/asset/host", Icon: "Cpu", Sort: 1, Status: 1},
		{ParentID: 8, Name: "凭证管理", Type: 2, Permission: "asset:credential:list", Path: "/asset/credential", Component: "views/asset/credential", Icon: "Key", Sort: 2, Status: 1},
		{ParentID: 8, Name: "主机分组", Type: 2, Permission: "asset:group:list", Path: "/asset/group", Component: "views/asset/group", Icon: "Folder", Sort: 3, Status: 1},
		{ParentID: 8, Name: "授权管理", Type: 2, Permission: "asset:auth:list", Path: "/asset/auth", Component: "views/asset/auth", Icon: "Connection", Sort: 4, Status: 1},

		{ParentID: 0, Name: "运维中心", Type: 1, Path: "/ops", Icon: "Terminal", Sort: 4, Status: 1},
		{ParentID: 13, Name: "Web终端", Type: 2, Permission: "ops:terminal:connect", Path: "/ops/terminal", Component: "views/ops/terminal", Icon: "VideoPlay", Sort: 1, Status: 1},
		{ParentID: 13, Name: "会话审计", Type: 2, Permission: "ops:session:list", Path: "/ops/session", Component: "views/ops/session", Icon: "Film", Sort: 2, Status: 1},
		{ParentID: 13, Name: "操作日志", Type: 2, Permission: "ops:audit:list", Path: "/ops/audit", Component: "views/ops/audit", Icon: "Document", Sort: 3, Status: 1},

		{ParentID: 0, Name: "安全中心", Type: 1, Path: "/security", Icon: "Lock", Sort: 5, Status: 1},
		{ParentID: 18, Name: "高危指令", Type: 2, Permission: "security:risk:list", Path: "/security/risk", Component: "views/security/risk", Icon: "Warning", Sort: 1, Status: 1},

		{ParentID: 0, Name: "任务编排", Type: 1, Path: "/task", Icon: "Calendar", Sort: 6, Status: 1},
		{ParentID: 21, Name: "脚本库", Type: 2, Permission: "task:script:list", Path: "/task/script", Component: "views/task/script", Icon: "Notebook", Sort: 1, Status: 1},
		{ParentID: 21, Name: "任务模板", Type: 2, Permission: "task:template:list", Path: "/task/template", Component: "views/task/template", Icon: "Files", Sort: 2, Status: 1},
		{ParentID: 21, Name: "任务执行", Type: 2, Permission: "task:execution:list", Path: "/task/execution", Component: "views/task/execution", Icon: "Promotion", Sort: 3, Status: 1},
	}
	database.DB.CreateInBatches(menus, 50)

	// 将全部菜单授权给 superadmin
	var super model.Role
	if err := database.DB.Where("code = ?", "superadmin").First(&super).Error; err == nil {
		var all []model.Menu
		database.DB.Find(&all)
		database.DB.Model(&super).Association("Menus").Replace(all)
	}
	log.Println("[seed] 已创建默认菜单与权限点")
}

func seedHighRisk() {
	var cnt int64
	database.DB.Model(&model.HighRiskCommand{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	rules := []model.HighRiskCommand{
		{Name: "递归删除根目录", Pattern: "rm -rf /", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "删除系统根目录"},
		{Name: "格式化磁盘", Pattern: "mkfs", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "格式化文件系统"},
		{Name: "dd写磁盘", Pattern: "dd if=", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "直接写设备"},
		{Name: "fork炸弹", Pattern: ":(){ :|:& };:", MatchType: "exact", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "耗尽进程资源"},
		{Name: "关机", Pattern: "shutdown", MatchType: "prefix", RiskLevel: "high", Action: "block", Enabled: 1, Builtin: 1, Remark: "关闭系统"},
		{Name: "重启", Pattern: "reboot", MatchType: "prefix", RiskLevel: "high", Action: "block", Enabled: 1, Builtin: 1, Remark: "重启系统"},
		{Name: "断电", Pattern: "poweroff", MatchType: "prefix", RiskLevel: "high", Action: "block", Enabled: 1, Builtin: 1, Remark: "断电关机"},
		{Name: "递归改权限", Pattern: "chmod -R 777 /", MatchType: "prefix", RiskLevel: "high", Action: "block", Enabled: 1, Builtin: 1, Remark: "放宽根目录权限"},
		{Name: "覆盖MBR", Pattern: "dd if=/dev/zero of=/dev/sda", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "清空磁盘引导"},
		{Name: "擦除文件系统签名", Pattern: "wipefs", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "擦除分区表"},
		{Name: "分区操作", Pattern: "fdisk", MatchType: "prefix", RiskLevel: "high", Action: "warn", Enabled: 1, Builtin: 1, Remark: "磁盘分区"},
		{Name: "切换运行级别", Pattern: "init 0", MatchType: "prefix", RiskLevel: "high", Action: "block", Enabled: 1, Builtin: 1, Remark: "进入停机级别"},
		{Name: "清空文件", Pattern: "> /dev/sda", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "向设备写入空数据"},
		{Name: "移动根目录", Pattern: "mv / /", MatchType: "prefix", RiskLevel: "critical", Action: "block", Enabled: 1, Builtin: 1, Remark: "移动根目录"},
	}
	database.DB.CreateInBatches(rules, 20)
	log.Println("[seed] 已创建 14 条内置高危指令规则")
}
