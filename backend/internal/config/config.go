package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Bastion  BastionConfig  `mapstructure:"bastion"`
	OSP      OSPConfig      `mapstructure:"osp"`
	Record   RecordConfig   `mapstructure:"record"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	Charset      string `mapstructure:"charset"`
	MaxIdleConns int    `mapstructure:"max-idle-conns"`
	MaxOpenConns int    `mapstructure:"max-open-conns"`
	LogMode      bool   `mapstructure:"log-mode"`
}

// DSN 拼装 MySQL 连接串
func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		m.Username, m.Password, m.Host, m.Port, m.Database, m.Charset)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire-hours"`
}

type BastionConfig struct {
	Port        int    `mapstructure:"port"`
	HostKeyPath string `mapstructure:"host-key-path"`
}

// OSPConfig OSP Agent 服务配置（执行机通过自定义端口加密回连）
type OSPConfig struct {
	// Enabled 是否启用 OSP 服务
	Enabled bool `mapstructure:"enabled"`
	// Port 自定义监听端口（非 SSH、非 HTTP，走自研 OSP 加密协议）
	Port int `mapstructure:"port"`
	// Heartbeat 下发给 Agent 的心跳周期（秒）
	Heartbeat int `mapstructure:"heartbeat"`
	// OfflineAfter 超过该秒数未心跳判定离线
	OfflineAfter int `mapstructure:"offline-after"`
	// HandshakeTimeout 握手超时（秒）
	HandshakeTimeout int `mapstructure:"handshake-timeout"`
	// DBThrottle 心跳写库节流间隔（秒）
	DBThrottle int `mapstructure:"db-throttle"`
}

// applyDefaults 补齐未配置项
func (o *OSPConfig) applyDefaults() {
	if o.Port == 0 {
		o.Port = 9090
	}
	if o.Heartbeat == 0 {
		o.Heartbeat = 10
	}
	if o.OfflineAfter == 0 {
		o.OfflineAfter = 35
	}
	if o.HandshakeTimeout == 0 {
		o.HandshakeTimeout = 20
	}
	if o.DBThrottle == 0 {
		o.DBThrottle = 5
	}
}

type RecordConfig struct {
	Path string `mapstructure:"path"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	Path  string `mapstructure:"path"`
}

// SecurityConfig 安全相关配置
type SecurityConfig struct {
	// IPWhitelistEnabled 是否启用 IP 白名单（开启后仅允许列表内 IP/网段访问页面与接口）
	IPWhitelistEnabled bool `mapstructure:"ip-whitelist-enabled"`
	// IPWhitelist 允许的 IP 或 CIDR 网段，例如 192.168.1.0/24、10.0.0.1
	IPWhitelist []string `mapstructure:"ip-whitelist"`
}

var Global Config

// Init 加载配置文件（默认 configs/config.yaml，可用 -c 指定）
func Init(path string) error {
	if path == "" {
		path = "configs/config.yaml"
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	// 支持环境变量覆盖（容器化部署用）：前缀 BOAT_，把嵌套点替换为下划线
	// 例如 BOAT_MYSQL_HOST=mysql 覆盖 mysql.host，BOAT_REDIS_PORT=6379 覆盖 redis.port
	v.SetEnvPrefix("BOAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, k := range []string{"mysql.host", "mysql.port", "redis.host", "redis.port"} {
		_ = v.BindEnv(k)
	}
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := v.Unmarshal(&Global); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	Global.OSP.applyDefaults()
	if bastionPort := Global.Bastion.Port; bastionPort == 0 {
		Global.Bastion.Port = 2222
	}
	return nil
}
