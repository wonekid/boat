package middleware

import (
	"boat/internal/config"
	"boat/internal/utils"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// IPWhitelist IP 白名单中间件。
// 在 security.ip-whitelist-enabled 开启且列表非空时，仅允许白名单内的 IP/网段
// 访问全部请求（包括前端页面静态资源与后端 API）。超级管理员不豁免——白名单是
// 网络层入口限制，此时请求尚未携带用户身份。
func IPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {
		sec := config.Global.Security
		if !sec.IPWhitelistEnabled || len(sec.IPWhitelist) == 0 {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if ipInWhitelist(ip, sec.IPWhitelist) {
			c.Next()
			return
		}
		utils.Forbidden(c, "IP 不在白名单，拒绝访问")
		c.Abort()
	}
}

// ipInWhitelist 判断客户端 IP 是否命中白名单（支持精确 IP 与 CIDR 网段）
func ipInWhitelist(ip string, list []string) bool {
	// 去掉可能的端口（如 RemoteAddr 形如 1.2.3.4:5678）
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	client := net.ParseIP(ip)
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == ip {
			return true
		}
		if _, cidr, err := net.ParseCIDR(item); err == nil {
			if client != nil && cidr.Contains(client) {
				return true
			}
		}
	}
	return false
}
