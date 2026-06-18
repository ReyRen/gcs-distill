package gcs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	TaskStateContainerDone = 7
	TaskStateBaseError     = 10
)

// Client 是 gcs-distill 到 gcs-v2 的唯一容器执行边界。
//
// gcs-distill 负责蒸馏业务、EasyDistill 配置和工作目录；gcs-v2 负责资源调度、
// XPU 软占用、镜像拉取、容器生命周期和日志状态。
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

type SelectedResource struct {
	NodeName    string `json:"node_name"`
	NodeAddress string `json:"node_address"`
	XPUIndices  []int  `json:"xpu_indices"`
}

type ContainerTaskRequest struct {
	TaskUID           int                `json:"task_uid"`
	TaskID            int                `json:"task_id"`
	ContainerName     string             `json:"container_name"`
	Image             string             `json:"image"`
	Command           string             `json:"command,omitempty"`
	Args              []string           `json:"args,omitempty"`
	WorkingDir        string             `json:"working_dir"`
	LogPath           string             `json:"log_path,omitempty"`
	Envs              string             `json:"envs,omitempty"`
	WorkersName       []string           `json:"workers_name,omitempty"`
	WorkerNums        int                `json:"worker_nums,omitempty"`
	XPUNums           int                `json:"xpu_nums,omitempty"`
	NodeType          string             `json:"node_type,omitempty"`
	SelectedResources []SelectedResource `json:"selected_resources,omitempty"`
}

type AcceptedResponse struct {
	Accepted      bool   `json:"accepted"`
	ContainerName string `json:"container_name"`
	TaskType      int    `json:"task_type"`
	Message       string `json:"message"`
}

type TaskInfo struct {
	TaskStates        int    `json:"task_states"`
	TaskContainerName string `json:"task_containername"`
	TaskLogPath       string `json:"task_log_path"`
	TaskTime          string `json:"task_time"`
}

func (c *Client) ListNodes(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/nodes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetNode(ctx context.Context, nodeName string) (map[string]any, bool, error) {
	nodes, err := c.ListNodes(ctx)
	if err != nil {
		return nil, false, err
	}

	items, ok := nodes["items"].([]any)
	if !ok {
		return nil, false, nil
	}
	for _, item := range items {
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"node_hostname", "node_name", "name", "node_id"} {
			if value, ok := node[key].(string); ok && value == nodeName {
				return node, true, nil
			}
		}
	}
	return nil, false, nil
}

func (c *Client) CreateContainerTask(ctx context.Context, req ContainerTaskRequest) (*AcceptedResponse, error) {
	var out AcceptedResponse
	if err := c.doJSON(ctx, http.MethodPost, "/tasks/container", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTask(ctx context.Context, containerName string) (*TaskInfo, bool, error) {
	var out TaskInfo
	found, err := c.doJSONMaybeNotFound(ctx, http.MethodGet, "/tasks/"+url.PathEscape(containerName), nil, &out)
	if err != nil || !found {
		return nil, found, err
	}
	return &out, true, nil
}

func (c *Client) DeleteTask(ctx context.Context, containerName string) error {
	return c.doJSON(ctx, http.MethodDelete, "/tasks/"+url.PathEscape(containerName), nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	found, err := c.doJSONMaybeNotFound(ctx, method, path, body, out)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("gcs-v2 returned 404 for %s %s", method, path)
	}
	return nil
}

func (c *Client) doJSONMaybeNotFound(ctx context.Context, method, path string, body any, out any) (bool, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return false, fmt.Errorf("encode gcs-v2 request failed: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return false, fmt.Errorf("create gcs-v2 request failed: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("call gcs-v2 failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("gcs-v2 returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return true, nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, fmt.Errorf("parse gcs-v2 response failed: %w", err)
	}
	return true, nil
}
