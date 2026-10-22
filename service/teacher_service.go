package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
)

type TeacherService struct {
	baseURL string
	client  *http.Client
}
type Teacher struct {
	ID               string `json:"id"`
	UID              string `json:"uid"`
	Name             string `json:"name"`
	ModelName        string `json:"served_model_name"`
	Status           string `json:"status"`
	ServiceType      string `json:"service_type"`
	SupportsVision   *bool  `json:"supports_vision"`
	CapabilitySource string `json:"capability_source"`
}

type teacherRecord struct {
	Teacher
	ModelPath string `json:"model_path"`
}

func NewTeacherService(baseURL string, timeout time.Duration) *TeacherService {
	return &TeacherService{strings.TrimRight(baseURL, "/"), &http.Client{Timeout: timeout}}
}

func (s *TeacherService) get(ctx context.Context, path string, target any) error {
	if s.baseURL == "" {
		return fmt.Errorf("model center is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("model center unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("model center returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, target)
}

func (s *TeacherService) List(ctx context.Context, uid int) ([]Teacher, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("uid must be positive")
	}
	items := []Teacher{}
	for page := 1; ; page++ {
		var result struct {
			Items []teacherRecord `json:"items"`
			Total int             `json:"total"`
		}
		path := fmt.Sprintf("/services?uid=%d&status=online&page=%d&page_size=100", uid, page)
		if err := s.get(ctx, path, &result); err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			if item.UID == strconv.Itoa(uid) && item.Status == "online" {
				item.CapabilitySource = "unknown"
				if item.ServiceType == "local_inference" && item.ModelPath != "" {
					if modelType, vision, _ := modelCapabilities(item.ModelPath); modelType != "" {
						item.SupportsVision = &vision
						item.CapabilitySource = "model_config"
					}
				}
				items = append(items, item.Teacher)
			}
		}
		if len(result.Items) < 100 || page*100 >= result.Total {
			break
		}
	}
	return items, nil
}

func (s *TeacherService) Resolve(ctx context.Context, uid int, model *types.ModelConfig) error {
	if model == nil {
		return fmt.Errorf("teacher is required")
	}
	if model.ServiceID == "" {
		return validateTeacherEndpoint(model)
	}
	// Verify ownership before requesting credentials from the connection API.
	var detail Teacher
	id := url.PathEscape(model.ServiceID)
	if err := s.get(ctx, "/services/"+id, &detail); err != nil {
		return err
	}
	if detail.UID != strconv.Itoa(uid) || detail.Status != "online" {
		return fmt.Errorf("teacher service is not online or does not belong to this user")
	}
	var conn struct {
		Status  string `json:"status"`
		BaseURL string `json:"openai_base_url"`
		Model   string `json:"served_model_name"`
		Key     string `json:"api_key"`
	}
	if err := s.get(ctx, "/services/"+id+"/connection", &conn); err != nil {
		return err
	}
	if conn.Status != "online" {
		return fmt.Errorf("teacher service is not online")
	}
	model.ProviderType = types.ProviderAPI
	model.ModelName = conn.Model
	model.Endpoint = conn.BaseURL
	model.APISecretRef = conn.Key
	return validateTeacherEndpoint(model)
}

func validateTeacherEndpoint(model *types.ModelConfig) error {
	u, err := url.Parse(strings.TrimSpace(model.Endpoint))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return fmt.Errorf("teacher requires an HTTP(S) base URL without URL credentials")
	}
	if model.ModelName == "" {
		return fmt.Errorf("teacher model_name is required")
	}
	model.Endpoint = strings.TrimSuffix(strings.TrimRight(model.Endpoint, "/"), "/chat/completions")
	model.ProviderType = types.ProviderAPI
	return nil
}

func (s *TeacherService) Test(ctx context.Context, uid int, model *types.ModelConfig) error {
	if err := s.Resolve(ctx, uid, model); err != nil {
		return err
	}
	base := strings.TrimRight(model.Endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return err
	}
	key := model.APISecretRef
	if key == "" {
		key, _ = model.ExtraParams["api_key"].(string)
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("teacher connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("teacher model discovery returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return err
	}
	for _, item := range result.Data {
		if item.ID == model.ModelName {
			payload, _ := json.Marshal(map[string]interface{}{"model": model.ModelName, "messages": []map[string]string{{"role": "user", "content": "Reply OK."}}, "max_tokens": 8, "stream": false})
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			if key != "" {
				req.Header.Set("Authorization", "Bearer "+key)
			}
			chat, err := s.client.Do(req)
			if err != nil {
				return fmt.Errorf("teacher inference failed: %w", err)
			}
			defer chat.Body.Close()
			if chat.StatusCode != http.StatusOK {
				return fmt.Errorf("teacher inference returned HTTP %d", chat.StatusCode)
			}
			var completion struct {
				Choices []struct {
					Message map[string]interface{} `json:"message"`
				} `json:"choices"`
			}
			if err := json.NewDecoder(io.LimitReader(chat.Body, 2<<20)).Decode(&completion); err != nil {
				return err
			}
			if len(completion.Choices) == 0 || completion.Choices[0].Message == nil {
				return fmt.Errorf("teacher returned no completion")
			}
			return nil
		}
	}
	return fmt.Errorf("selected model is not advertised by teacher /models")
}
