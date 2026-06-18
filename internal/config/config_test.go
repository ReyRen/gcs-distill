package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelCenterStyleDatabaseConfig(t *testing.T) {
	t.Setenv("GCS_DISTILL_TEST_DB_PASSWORD", "secret")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	content := `
[server]
host = "127.0.0.1"
port = 18080
mode = "debug"

[database]
enabled = true
driver = "mysql"
host = "db.example"
port = 3306
name = "ai_market"
user = "root"
password = ""
password_env = "GCS_DISTILL_TEST_DB_PASSWORD"
max_open_conns = 30
max_idle_conns = 7
conn_max_lifetime_seconds = 600

[storage]
type = "nfs"
base_path = "/mnt/shared/distill"
models_base_path = "/mnt/shared/distill/models"

[gcs]
base_url = "http://gcs-v2:8072/api/v1/"
timeout_seconds = 12

[logging]
level = "debug"
output = "stdout"
file_path = "/tmp/gcs-distill.log"
max_size = 10
max_age = 2
compress = false

[executor]
workspace_root = "/mnt/shared/distill"
max_concurrent = 3
runtime_image = "gcs-distill/easydistill:test"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.Driver != "mysql" {
		t.Fatalf("database driver = %q, want mysql", cfg.Database.Driver)
	}
	if cfg.Database.Name != "ai_market" {
		t.Fatalf("database name = %q, want ai_market", cfg.Database.Name)
	}
	if got := cfg.Database.ResolvePassword(); got != "secret" {
		t.Fatalf("resolved password = %q, want secret", got)
	}
	if cfg.GCS.BaseURL != "http://gcs-v2:8072/api/v1" {
		t.Fatalf("gcs base url = %q", cfg.GCS.BaseURL)
	}
	if cfg.Executor.RuntimeImage != "gcs-distill/easydistill:test" {
		t.Fatalf("runtime image = %q", cfg.Executor.RuntimeImage)
	}
}

func TestLoadRejectsNonMySQLDatabase(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	content := `
[database]
enabled = true
driver = "sqlite"
host = "127.0.0.1"
port = 3306
name = "ai_market"
user = "root"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(configPath); err == nil {
		t.Fatal("Load() error = nil, want non-mysql validation error")
	}
}
