package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Storage  StorageConfig  `yaml:"storage"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	Logging  LoggingConfig  `yaml:"logging"`
	Executor ExecutorConfig `yaml:"executor"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Host string `yaml:"host"` // 服务地址
	Port int    `yaml:"port"` // 服务端口
	Mode string `yaml:"mode"` // 运行模式: debug, release, test
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`     // 数据库主机
	Port     int    `yaml:"port"`     // 数据库端口
	User     string `yaml:"user"`     // 用户名
	Password string `yaml:"password"` // 密码
	DBName   string `yaml:"dbname"`   // 数据库名
	SSLMode  string `yaml:"sslmode"`  // SSL 模式
	MaxConns int    `yaml:"max_conns"` // 最大连接数
	MinConns int    `yaml:"min_conns"` // 最小连接数
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `yaml:"host"`     // Redis 主机
	Port     int    `yaml:"port"`     // Redis 端口
	Password string `yaml:"password"` // 密码
	DB       int    `yaml:"db"`       // 数据库编号
	PoolSize int    `yaml:"pool_size"` // 连接池大小
}

// StorageConfig 共享存储配置
type StorageConfig struct {
	Type     string `yaml:"type"`      // 存储类型: nfs, ceph, local
	BasePath string `yaml:"base_path"` // 基础路径
}

// GRPCConfig gRPC 配置
type GRPCConfig struct {
	Port            int `yaml:"port"`             // gRPC 端口
	MaxRecvMsgSize  int `yaml:"max_recv_msg_size"` // 最大接收消息大小 (MB)
	MaxSendMsgSize  int `yaml:"max_send_msg_size"` // 最大发送消息大小 (MB)
	ConnectionTimeout int `yaml:"connection_timeout"` // 连接超时 (秒)
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level    string `yaml:"level"`     // 日志级别: debug, info, warn, error
	Output   string `yaml:"output"`    // 输出: stdout, file
	FilePath string `yaml:"file_path"` // 日志文件路径
	MaxSize  int    `yaml:"max_size"`  // 单个日志文件最大大小 (MB)
	MaxAge   int    `yaml:"max_age"`   // 日志文件保留天数
	Compress bool   `yaml:"compress"`  // 是否压缩旧日志
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	WorkspaceRoot string `yaml:"workspace_root"` // 工作空间根目录
	MaxConcurrent int    `yaml:"max_concurrent"` // 最大并发执行数
}

// Load 从文件加载配置
func Load(configPath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	// Server 默认值
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Mode == "" {
		config.Server.Mode = "release"
	}

	// Database 默认值
	if config.Database.Port == 0 {
		config.Database.Port = 5432
	}
	if config.Database.SSLMode == "" {
		config.Database.SSLMode = "disable"
	}
	if config.Database.MaxConns == 0 {
		config.Database.MaxConns = 25
	}
	if config.Database.MinConns == 0 {
		config.Database.MinConns = 5
	}

	// Redis 默认值
	if config.Redis.Port == 0 {
		config.Redis.Port = 6379
	}
	if config.Redis.PoolSize == 0 {
		config.Redis.PoolSize = 10
	}

	// GRPC 默认值
	if config.GRPC.Port == 0 {
		config.GRPC.Port = 50051
	}
	if config.GRPC.MaxRecvMsgSize == 0 {
		config.GRPC.MaxRecvMsgSize = 10 // 10 MB
	}
	if config.GRPC.MaxSendMsgSize == 0 {
		config.GRPC.MaxSendMsgSize = 10 // 10 MB
	}
	if config.GRPC.ConnectionTimeout == 0 {
		config.GRPC.ConnectionTimeout = 30 // 30 秒
	}

	// Logging 默认值
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Output == "" {
		config.Logging.Output = "stdout"
	}
	if config.Logging.MaxSize == 0 {
		config.Logging.MaxSize = 100 // 100 MB
	}
	if config.Logging.MaxAge == 0 {
		config.Logging.MaxAge = 7 // 7 天
	}

	// Executor 默认值
	if config.Executor.WorkspaceRoot == "" {
		config.Executor.WorkspaceRoot = "/shared"
	}
	if config.Executor.MaxConcurrent <= 0 {
		config.Executor.MaxConcurrent = 5
	}
}

// validate 验证配置
func validate(config *Config) error {
	// 验证 Server
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("无效的服务端口: %d", config.Server.Port)
	}
	if config.Server.Mode != "debug" && config.Server.Mode != "release" && config.Server.Mode != "test" {
		return fmt.Errorf("无效的运行模式: %s", config.Server.Mode)
	}

	// 验证 Database
	if config.Database.Host == "" {
		return fmt.Errorf("数据库主机不能为空")
	}
	if config.Database.User == "" {
		return fmt.Errorf("数据库用户名不能为空")
	}
	if config.Database.DBName == "" {
		return fmt.Errorf("数据库名不能为空")
	}

	// 验证 Redis
	if config.Redis.Host == "" {
		return fmt.Errorf("Redis 主机不能为空")
	}

	// 验证 Storage
	if config.Storage.BasePath == "" {
		return fmt.Errorf("存储基础路径不能为空")
	}

	// 验证 Logging
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[config.Logging.Level] {
		return fmt.Errorf("无效的日志级别: %s", config.Logging.Level)
	}

	return nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// GetRedisAddr 获取 Redis 地址
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
