package scheduler

import (
	"boat/internal/database"
	"boat/internal/model"
	"encoding/json"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Executor 实际执行函数（由 controller.LaunchExecution 注入，避免循环依赖）
var Executor func(execType string, hostIDs []uint, cmd string, uid uint, uname string, name string, templateID uint, credentialID uint, needRoot bool) uint

var (
	mu      sync.Mutex
	c       *cron.Cron
	entries = map[uint]cron.EntryID{}
)

// Init 启动调度器并加载所有启用中的定时任务
func Init() {
	c = cron.New()
	loadEnabled()
	c.Start()
}

func loadEnabled() {
	var list []model.TaskSchedule
	database.DB.Where("enabled = ?", 1).Find(&list)
	for _, ts := range list {
		_ = Register(ts)
	}
}

// ValidateCron 校验 cron 表达式（标准 5 段：分 时 日 月 周）
func ValidateCron(expr string) error {
	_, err := cron.ParseStandard(expr)
	return err
}

// Register 注册（或重启）一个定时任务，并刷新下次运行时间
func Register(ts model.TaskSchedule) error {
	if err := ValidateCron(ts.Cron); err != nil {
		return err
	}
	mu.Lock()
	if old, ok := entries[ts.ID]; ok {
		c.Remove(old)
		delete(entries, ts.ID)
	}
	id, err := c.AddFunc(ts.Cron, func() { fire(ts) })
	if err != nil {
		mu.Unlock()
		return err
	}
	entries[ts.ID] = id
	mu.Unlock()

	if sch, e := cron.ParseStandard(ts.Cron); e == nil {
		next := sch.Next(time.Now())
		database.DB.Model(&model.TaskSchedule{}).Where("id = ?", ts.ID).Update("next_run_at", &next)
	}
	return nil
}

// Reload 重新加载（更新后调用：先注销再按启用状态注册）
func Reload(ts model.TaskSchedule) {
	Unregister(ts.ID)
	if ts.Enabled == 1 {
		Register(ts)
	}
}

// Unregister 注销定时任务
func Unregister(id uint) {
	mu.Lock()
	if eid, ok := entries[id]; ok {
		c.Remove(eid)
		delete(entries, id)
	}
	mu.Unlock()
	database.DB.Model(&model.TaskSchedule{}).Where("id = ?", id).Update("next_run_at", nil)
}

// RunNow 立即执行一次，返回执行记录 ID
func RunNow(ts model.TaskSchedule) uint {
	return fireWithResult(ts)
}

func parseHostIDs(s string) []uint {
	var ids []uint
	if s == "" {
		return ids
	}
	_ = json.Unmarshal([]byte(s), &ids)
	return ids
}

func resolveUser(uname string) (uint, string) {
	if uname == "" {
		return 1, "system"
	}
	var u model.User
	if err := database.DB.Where("username = ?", uname).First(&u).Error; err == nil {
		return u.ID, u.Username
	}
	return 1, uname
}

// fire 触发执行，并记录上次运行时间、异步回填执行结果
func fire(ts model.TaskSchedule) {
	execID := fireWithResult(ts)
	now := time.Now()
	database.DB.Model(&model.TaskSchedule{}).Where("id = ?", ts.ID).Updates(map[string]interface{}{
		"last_run_at": &now, "status": "running",
	})
	go func() {
		time.Sleep(2 * time.Second)
		for i := 0; i < 45; i++ {
			var e model.TaskExecution
			if database.DB.First(&e, execID).Error == nil && e.Status != "running" {
				database.DB.Model(&model.TaskSchedule{}).Where("id = ?", ts.ID).Updates(map[string]interface{}{
					"status":      "idle",
					"last_result": e.Result,
				})
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

func fireWithResult(ts model.TaskSchedule) uint {
	cmd := buildCommand(ts)
	hostIDs := parseHostIDs(ts.HostIDs)
	uid, uname := resolveUser(ts.CreatedBy)
	if Executor == nil {
		return 0
	}
	var templateID, credentialID uint
	if ts.TemplateID > 0 {
		templateID = ts.TemplateID
		var tpl model.TaskTemplate
		if err := database.DB.First(&tpl, ts.TemplateID).Error; err == nil {
			credentialID = tpl.CredentialID
		}
	}
	return Executor(ts.Type, hostIDs, cmd, uid, uname, "定时-"+ts.Name, templateID, credentialID, ts.NeedRoot)
}

func buildCommand(ts model.TaskSchedule) string {
	cmd := ts.Command
	if ts.Type == "script" && ts.ScriptID > 0 {
		var s model.Script
		if err := database.DB.First(&s, ts.ScriptID).Error; err == nil {
			if s.Lang == "python" {
				cmd = "python3 - <<'BOATEOF'\n" + s.Content + "\nBOATEOF"
			} else {
				cmd = s.Content
			}
		}
	}
	return cmd
}
