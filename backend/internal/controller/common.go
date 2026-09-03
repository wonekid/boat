package controller

import (
	"encoding/json"
	"strconv"

	"boat/internal/middleware"

	"github.com/gin-gonic/gin"
)

// middlewareUser 取当前用户（封装中间件取值）
func middlewareUser(c *gin.Context) (uint, string) {
	return middleware.GetUser(c)
}

// clientIP 取客户端 IP
func clientIP(c *gin.Context) string {
	return c.ClientIP()
}

// itoa 整型转字符串（避免与 main 包的 itoa 冲突）
func itoa(i int) string {
	return strconv.Itoa(i)
}

// jsonUnmarshal 便捷反序列化（忽略错误时由调用方决定）
func jsonUnmarshal(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}
