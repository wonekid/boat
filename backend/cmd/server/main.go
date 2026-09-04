package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"boat/internal/bastion"
	"boat/internal/casbin"
	"boat/internal/config"
	"boat/internal/controller"
	"boat/internal/database"
	"boat/internal/middleware"
	"boat/internal/osp"
	"boat/internal/router"
	"boat/internal/scheduler"
	"boat/internal/service"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	configPath := flag.String("c", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	if err := config.Init(*configPath); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// RSA 密钥（用于凭证加密）
	if err := utils.InitRSA("configs/rsa_key"); err != nil {
		log.Fatalf("RSA 初始化失败: %v", err)
	}

	// 数据库
	if err := database.Init(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("表结构迁移失败: %v", err)
	}

	// 种子数据
	if err := service.Seed(); err != nil {
		log.Fatalf("种子数据失败: %v", err)
	}

	// Casbin 权限
	if err := casbin.Init(); err != nil {
		log.Fatalf("Casbin 初始化失败: %v", err)
	}

	// 定时任务调度器（注入执行器，加载启用中的任务）
	scheduler.Executor = controller.LaunchExecution
	scheduler.Init()

	// OSP Agent 服务：自定义端口 + 自研加密协议，执行机 agent 反向回连，
	// 用于下发命令/脚本任务与实时节点监控（SSH 不可登录时的应急控制通道）
	if config.Global.OSP.Enabled {
		osp.Init(osp.Options{
			Port:             config.Global.OSP.Port,
			Heartbeat:        config.Global.OSP.Heartbeat,
			OfflineAfter:     config.Global.OSP.OfflineAfter,
			HandshakeTimeout: config.Global.OSP.HandshakeTimeout,
			DBThrottle:       config.Global.OSP.DBThrottle,
			Enabled:          true,
		})
		go func() {
			if err := osp.Default().Start(); err != nil {
				log.Printf("[osp] OSP 服务启动失败: %v", err)
			}
		}()
	}

	// HTTP 服务
	gin.SetMode(config.Global.Server.Mode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.IPWhitelist())
	router.Register(r)

	// 可选：托管前端构建产物（web/dist 存在时）
	if dist := filepath.Join("..", "web", "dist"); dirExists(dist) {
		r.Static("/assets", filepath.Join(dist, "assets"))
		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(dist, "index.html"))
		})
		log.Printf("[server] 已托管前端静态资源: %s", dist)
	}

	// 独立 SSH 堡垒机（端口 2222），失败不影响 HTTP 服务
	go func() {
		if err := bastion.Start(); err != nil {
			log.Printf("[bastion] 启动失败: %v", err)
		}
	}()

	addr := ":" + itoa(config.Global.Server.Port)
	log.Printf("[server] boat 运维平台启动于 http://127.0.0.1%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
