package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
	GCS      GCSConfig
	Logging  LoggingConfig
	Executor ExecutorConfig
}

type ServerConfig struct {
	Host string
	Port int
	Mode string
}

type DatabaseConfig struct {
	Enabled                bool
	Driver                 string
	Host                   string
	Port                   int
	Name                   string
	User                   string
	Password               string
	PasswordEnv            string
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
}

func (c DatabaseConfig) ResolvePassword() string {
	if c.Password != "" {
		return c.Password
	}
	if c.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.PasswordEnv)
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local&charset=utf8mb4",
		c.User, c.ResolvePassword(), c.Host, c.Port, c.Name)
}

func (c DatabaseConfig) ConnMaxLifetime() time.Duration {
	seconds := c.ConnMaxLifetimeSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

type StorageConfig struct {
	Type           string
	RootPath       string
	ModelsBasePath string
}

type GCSConfig struct {
	BaseURL        string
	TimeoutSeconds int
}

type LoggingConfig struct {
	Level    string
	Output   string
	FilePath string
	MaxSize  int
	MaxAge   int
	Compress bool
}

type ExecutorConfig struct {
	MaxConcurrent int
	RuntimeImage  string
}

func Load(configPath string) (*Config, error) {
	cfg := Default()
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if err := applyValue(cfg, section, key, value); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config file failed: %w", err)
	}
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config failed: %w", err)
	}
	return cfg, nil
}

func Default() *Config {
	cfg := &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080, Mode: "release"},
		Database: DatabaseConfig{
			Enabled:                true,
			Driver:                 "mysql",
			Host:                   "172.18.127.67",
			Port:                   3306,
			Name:                   "ai_market",
			User:                   "root",
			Password:               "!Market4AI",
			MaxOpenConns:           20,
			MaxIdleConns:           5,
			ConnMaxLifetimeSeconds: 300,
		},
		Storage: StorageConfig{
			Type:           "nfs",
			RootPath:       "/storage-root-jfs",
			ModelsBasePath: "/storage-root-jfs/train-base-models",
		},
		GCS: GCSConfig{BaseURL: "http://172.18.29.80:8072/api/v1", TimeoutSeconds: 10},
		Logging: LoggingConfig{
			Level:    "info",
			Output:   "stdout",
			FilePath: "/var/log/gcs-distill/server.log",
			MaxSize:  100,
			MaxAge:   7,
			Compress: true,
		},
		Executor: ExecutorConfig{
			MaxConcurrent: 5,
			RuntimeImage:  "easy-distill/easydistill:latest",
		},
	}
	normalizeStorage(&cfg.Storage)
	return cfg
}

func validate(config *Config) error {
	normalizeStorage(&config.Storage)

	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server.port: %d", config.Server.Port)
	}
	if config.Server.Mode != "debug" && config.Server.Mode != "release" && config.Server.Mode != "test" {
		return fmt.Errorf("invalid server.mode: %s", config.Server.Mode)
	}
	if !config.Database.Enabled {
		return fmt.Errorf("database.enabled must be true because gcs-distill requires persistent state")
	}
	if config.Database.Driver != "mysql" {
		return fmt.Errorf("database.driver must be mysql: %s", config.Database.Driver)
	}
	if config.Database.Host == "" {
		return fmt.Errorf("database.host must not be empty")
	}
	if config.Database.User == "" {
		return fmt.Errorf("database.user must not be empty")
	}
	if config.Database.Name == "" {
		return fmt.Errorf("database.name must not be empty")
	}
	if config.Storage.RootPath == "" {
		return fmt.Errorf("storage.root_path must not be empty")
	}
	if config.Storage.ModelsBasePath == "" {
		return fmt.Errorf("storage.models_base_path must not be empty")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[config.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s", config.Logging.Level)
	}
	return nil
}

func normalizeStorage(storage *StorageConfig) {
	storage.RootPath = strings.TrimRight(strings.TrimSpace(storage.RootPath), "/")
	storage.ModelsBasePath = strings.TrimRight(strings.TrimSpace(storage.ModelsBasePath), "/")
}

func (s StorageConfig) UserRoot(uid int) (string, error) {
	if uid <= 0 {
		return "", fmt.Errorf("uid must be greater than 0")
	}
	root := strings.TrimRight(strings.TrimSpace(s.RootPath), "/")
	if root == "" {
		root = "/storage-root-jfs"
	}
	return filepath.Clean(filepath.Join(root, "user-"+strconv.Itoa(uid))), nil
}

func (s StorageConfig) UserModelDistillBase(uid int) (string, error) {
	root, err := s.UserRoot(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "train-center", "model-distill"), nil
}

func (s StorageConfig) UserDatasetsBase(uid int) (string, error) {
	base, err := s.UserModelDistillBase(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "datasets"), nil
}

func (s StorageConfig) UserDatasetCandidates(uid int) (string, error) {
	base, err := s.UserDatasetsBase(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "candidates"), nil
}

func (s StorageConfig) UserDatasetUploads(uid int) (string, error) {
	base, err := s.UserDatasetsBase(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "uploaded"), nil
}

func (s StorageConfig) UserDatasetGenerated(uid int) (string, error) {
	base, err := s.UserDatasetsBase(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "generated"), nil
}

func applyValue(cfg *Config, section, key, value string) error {
	switch section + "." + key {
	case "server.host":
		cfg.Server.Host = value
	case "server.port":
		return setInt(value, &cfg.Server.Port, "server.port")
	case "server.mode":
		cfg.Server.Mode = value
	case "database.enabled":
		return setBool(value, &cfg.Database.Enabled, "database.enabled")
	case "database.driver":
		cfg.Database.Driver = value
	case "database.host":
		cfg.Database.Host = value
	case "database.port":
		return setInt(value, &cfg.Database.Port, "database.port")
	case "database.name":
		cfg.Database.Name = value
	case "database.user":
		cfg.Database.User = value
	case "database.password":
		cfg.Database.Password = value
	case "database.password_env":
		cfg.Database.PasswordEnv = value
	case "database.max_open_conns":
		return setInt(value, &cfg.Database.MaxOpenConns, "database.max_open_conns")
	case "database.max_idle_conns":
		return setInt(value, &cfg.Database.MaxIdleConns, "database.max_idle_conns")
	case "database.conn_max_lifetime_seconds":
		return setInt(value, &cfg.Database.ConnMaxLifetimeSeconds, "database.conn_max_lifetime_seconds")
	case "storage.type":
		cfg.Storage.Type = value
	case "storage.root_path":
		cfg.Storage.RootPath = value
	case "storage.models_base_path":
		cfg.Storage.ModelsBasePath = value
	case "gcs.base_url":
		cfg.GCS.BaseURL = strings.TrimRight(value, "/")
	case "gcs.timeout_seconds":
		return setInt(value, &cfg.GCS.TimeoutSeconds, "gcs.timeout_seconds")
	case "logging.level":
		cfg.Logging.Level = value
	case "logging.output":
		cfg.Logging.Output = value
	case "logging.file_path":
		cfg.Logging.FilePath = value
	case "logging.max_size":
		return setInt(value, &cfg.Logging.MaxSize, "logging.max_size")
	case "logging.max_age":
		return setInt(value, &cfg.Logging.MaxAge, "logging.max_age")
	case "logging.compress":
		return setBool(value, &cfg.Logging.Compress, "logging.compress")
	case "executor.max_concurrent":
		return setInt(value, &cfg.Executor.MaxConcurrent, "executor.max_concurrent")
	case "executor.runtime_image":
		cfg.Executor.RuntimeImage = value
	}
	return nil
}

func setInt(value string, target *int, name string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer: %w", name, err)
	}
	*target = parsed
	return nil
}

func setBool(value string, target *bool, name string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	*target = parsed
	return nil
}
