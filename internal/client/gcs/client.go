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

type logResponse struct {
	ContainerName string `json:"container_name"`
	Path          string `json:"path"`
	Tail          int    `json:"tail"`
	Content       string `json:"content"`
}

func (c *Client) GetBrain(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/brain", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListNodes(ctx context.Context) (any, error) {
	var out any
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

	for _, item := range nodeItems(nodes) {
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

func nodeItems(raw any) []any {
	switch typed := raw.(type) {
	case []any:
		return typed
	case map[string]any:
		if items, ok := typed["items"].([]any); ok {
			return items
		}
		if data, ok := typed["data"].(map[string]any); ok {
			if items, ok := data["items"].([]any); ok {
				return items
			}
			if nodes, ok := data["nodes"].([]any); ok {
				return nodes
			}
		}
	}
	return nil
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

func (c *Client) GetTaskLogs(ctx context.Context, containerName string, tail string) ([]byte, error) {
	path := "/tasks/" + url.PathEscape(containerName) + "/logs"
	query := url.Values{}
	if tail != "" {
		query.Set("tail", tail)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := c.doBytes(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	var decoded logResponse
	if err := json.Unmarshal(data, &decoded); err == nil && (decoded.Content != "" || decoded.ContainerName != "" || decoded.Path != "") {
		return []byte(decoded.Content), nil
	}
	return data, nil
}

func (c *Client) TaskLogsWebSocketURL(containerName string, tail string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid gcs base url: %w", err)
	}
	switch base.Scheme {
	case "http":
		base.Scheme = "ws"
	case "https":
		base.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported gcs scheme: %s", base.Scheme)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/tasks/" + url.PathEscape(containerName) + "/logs/ws"
	query := base.Query()
	if tail != "" {
		query.Set("tail", tail)
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
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

func (c *Client) doBytes(ctx context.Context, method, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create gcs-v2 request failed: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call gcs-v2 failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("gcs-v2 returned 404 for %s %s", method, path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gcs-v2 returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}
