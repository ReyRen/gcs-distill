package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

type DB struct {
	sql *sql.DB
}

func NewDB(cfg *config.DatabaseConfig) (*DB, error) {
	if cfg.Driver != "mysql" {
		return nil, fmt.Errorf("database driver must be mysql: %s", cfg.Driver)
	}

	sqlDB, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open MySQL connection failed: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect MySQL failed: %w", err)
	}

	wrapped := &DB{sql: sqlDB}
	if err := wrapped.Migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize MySQL schema failed: %w", err)
	}

	logger.Info("MySQL database connected",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
	)
	return wrapped, nil
}

func (db *DB) Close() {
	if db != nil && db.sql != nil {
		_ = db.sql.Close()
		logger.Info("MySQL database connection closed")
	}
}

func (db *DB) Ping(ctx context.Context) error {
	return db.sql.PingContext(ctx)
}

func (db *DB) Stats() sql.DBStats {
	return db.sql.Stats()
}
