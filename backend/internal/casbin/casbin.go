package casbin

import (
	"boat/internal/database"
	"boat/internal/model"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
)

var Enforcer *casbin.Enforcer

// modelConf Casbin 模型：RBAC + 路径通配
const modelConf = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`

// Init 初始化 Casbin（gorm-adapter 复用业务库）
func Init() error {
	adapter, err := gormadapter.NewAdapterByDB(database.DB)
	if err != nil {
		return err
	}
	// 用显式 model.Model 调用带类型签名的 NewEnforcer(m, a)，
	// 避免可变参数重载把 []byte 误断言为 model.Model 而 panic。
	m, err := casbinmodel.NewModelFromString(modelConf)
	if err != nil {
		return err
	}
	enf, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return err
	}
	Enforcer = enf
	return SyncFromDB()
}

// SyncFromDB 从数据库角色/菜单/用户角色重建 Casbin 策略
// p = role:code, menu.path, menu.permission
// g = username, role:code
func SyncFromDB() error {
	if Enforcer == nil {
		return nil
	}
	// 清空旧策略后重建（保留内置需手动管理时可调整）
	Enforcer.ClearPolicy()

	var roles []model.Role
	if err := database.DB.Preload("Menus").Find(&roles).Error; err != nil {
		return err
	}
	for _, role := range roles {
		roleSub := "role:" + role.Code
		for _, menu := range role.Menus {
			if menu.Path == "" || menu.Permission == "" {
				continue
			}
			_, _ = Enforcer.AddPolicy(roleSub, menu.Path, menu.Permission)
		}
	}

	var users []model.User
	if err := database.DB.Preload("Roles").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		for _, role := range user.Roles {
			_, _ = Enforcer.AddGroupingPolicy(user.Username, "role:"+role.Code)
		}
	}
	return Enforcer.SavePolicy()
}

// Check 校验用户是否有权限
func Check(username, obj, act string) (bool, error) {
	if Enforcer == nil {
		return true, nil
	}
	return Enforcer.Enforce(username, obj, act)
}
