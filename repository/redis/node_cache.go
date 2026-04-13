package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
)

// NodeCache Worker 节点缓存接口
type NodeCache interface {
	// SetNode 设置节点信息
	SetNode(ctx context.Context, node *types.WorkerNode) error
	// GetNode 获取节点信息
	GetNode(ctx context.Context, nodeName string) (*types.WorkerNode, error)
	// ListNodes 列出所有在线节点
	ListNodes(ctx context.Context) ([]*types.WorkerNode, error)
	// DeleteNode 删除节点信息
	DeleteNode(ctx context.Context, nodeName string) error
	// UpdateHeartbeat 更新节点心跳
	UpdateHeartbeat(ctx context.Context, nodeName string) error
	// CheckExpiredNodes 检查过期节点
	CheckExpiredNodes(ctx context.Context, timeout time.Duration) ([]string, error)
}

const (
	// nodeKeyPrefix 节点信息键前缀
	nodeKeyPrefix = "gcs:node:"
	// nodeSetKey 节点集合键
	nodeSetKey = "gcs:nodes"
	// nodeTTL 节点信息过期时间 (5 分钟)
	nodeTTL = 5 * time.Minute
)

// nodeCache Worker 节点缓存实现
type nodeCache struct {
	client *Client
}

// NewNodeCache 创建节点缓存
func NewNodeCache(client *Client) NodeCache {
	return &nodeCache{client: client}
}

// SetNode 设置节点信息
func (c *nodeCache) SetNode(ctx context.Context, node *types.WorkerNode) error {
	// 序列化节点信息
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("序列化节点信息失败: %w", err)
	}

	// 设置节点信息
	key := nodeKeyPrefix + node.NodeName
	if err := c.client.Set(ctx, key, data, nodeTTL); err != nil {
		return fmt.Errorf("设置节点信息失败: %w", err)
	}

	// 添加到节点集合
	rdb := c.client.GetClient()
	if err := rdb.SAdd(ctx, nodeSetKey, node.NodeName).Err(); err != nil {
		return fmt.Errorf("添加到节点集合失败: %w", err)
	}

	return nil
}

// GetNode 获取节点信息
func (c *nodeCache) GetNode(ctx context.Context, nodeName string) (*types.WorkerNode, error) {
	key := nodeKeyPrefix + nodeName

	// 获取节点信息
	data, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取节点信息失败: %w", err)
	}

	// 反序列化
	var node types.WorkerNode
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, fmt.Errorf("反序列化节点信息失败: %w", err)
	}

	return &node, nil
}

// ListNodes 列出所有在线节点
func (c *nodeCache) ListNodes(ctx context.Context) ([]*types.WorkerNode, error) {
	rdb := c.client.GetClient()

	// 获取所有节点名称
	nodeNames, err := rdb.SMembers(ctx, nodeSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	// 获取每个节点的详细信息
	var nodes []*types.WorkerNode
	for _, nodeName := range nodeNames {
		node, err := c.GetNode(ctx, nodeName)
		if err != nil {
			// 如果节点已过期，从集合中删除
			_ = rdb.SRem(ctx, nodeSetKey, nodeName)
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// DeleteNode 删除节点信息
func (c *nodeCache) DeleteNode(ctx context.Context, nodeName string) error {
	key := nodeKeyPrefix + nodeName

	// 删除节点信息
	if err := c.client.Del(ctx, key); err != nil {
		return fmt.Errorf("删除节点信息失败: %w", err)
	}

	// 从节点集合中删除
	rdb := c.client.GetClient()
	if err := rdb.SRem(ctx, nodeSetKey, nodeName).Err(); err != nil {
		return fmt.Errorf("从节点集合删除失败: %w", err)
	}

	return nil
}

// UpdateHeartbeat 更新节点心跳
func (c *nodeCache) UpdateHeartbeat(ctx context.Context, nodeName string) error {
	key := nodeKeyPrefix + nodeName

	// 更新过期时间
	if err := c.client.Expire(ctx, key, nodeTTL); err != nil {
		return fmt.Errorf("更新节点心跳失败: %w", err)
	}

	return nil
}

// CheckExpiredNodes 检查过期节点
func (c *nodeCache) CheckExpiredNodes(ctx context.Context, timeout time.Duration) ([]string, error) {
	rdb := c.client.GetClient()

	// 获取所有节点名称
	nodeNames, err := rdb.SMembers(ctx, nodeSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	var expiredNodes []string
	now := time.Now()

	// 检查每个节点的心跳时间
	for _, nodeName := range nodeNames {
		node, err := c.GetNode(ctx, nodeName)
		if err != nil {
			// 节点信息不存在，标记为过期
			expiredNodes = append(expiredNodes, nodeName)
			_ = rdb.SRem(ctx, nodeSetKey, nodeName)
			continue
		}

		// 检查心跳时间
		if now.Sub(node.LastHeartbeat) > timeout {
			expiredNodes = append(expiredNodes, nodeName)
			_ = c.DeleteNode(ctx, nodeName)
		}
	}

	return expiredNodes, nil
}
