package middleware

import (
	"boat/internal/casbin"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// CasbinRBAC 权限校验中间件
// 超级管理员（user id == 1）豁免
func CasbinRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, username := GetUser(c)
		if uid == 1 {
			// 超级管理员豁免
			c.Next()
			return
		}
		obj := c.Request.URL.Path
		act := c.Request.Method
		ok, err := casbin.Check(username, obj, act)
		if err != nil {
			utils.Forbidden(c, "权限校验异常")
			c.Abort()
			return
		}
		if !ok {
			utils.Forbidden(c, "无访问权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
