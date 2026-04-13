package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Client Redis 客户端
type Client struct {
	client *redis.Client
}

// NewClient 创建 Redis 客户端
func NewClient(cfg *config.RedisConfig) (*Client, error) {
	// 创建 Redis 客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.PoolSize / 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接测试失败: %w", err)
	}

	logger.Info("Redis 连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.Int("db", cfg.DB),
	)

	return &Client{client: rdb}, nil
}

// Close 关闭 Redis 连接
func (c *Client) Close() error {
	if c.client != nil {
		err := c.client.Close()
		if err == nil {
			logger.Info("Redis 连接已关闭")
		}
		return err
	}
	return nil
}

// GetClient 获取原生 Redis 客户端
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// Ping 测试 Redis 连接
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Set 设置键值
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取键值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// Del 删除键
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// HSet 设置哈希字段
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.client.HSet(ctx, key, values...).Err()
}

// HGet 获取哈希字段
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

// HGetAll 获取所有哈希字段
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// HDel 删除哈希字段
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}
