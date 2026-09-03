package middleware

import (
	"boat/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuth 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			utils.Unauthorized(c, "未携带令牌")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		tokenStr = strings.TrimSpace(tokenStr)
		claims, err := utils.ParseToken(tokenStr)
		if err != nil {
			utils.Unauthorized(c, "令牌无效或已过期")
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetUser 从上下文取当前用户
func GetUser(c *gin.Context) (uint, string) {
	uid, _ := c.Get("userID")
	uname, _ := c.Get("username")
	uidVal, _ := uid.(uint)
	unameVal, _ := uname.(string)
	return uidVal, unameVal
}
