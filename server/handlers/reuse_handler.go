package handlers

import (
	"encoding/json"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"strings"
)

// ClonePipeline reuses the server-side snapshot without exposing its credentials.
func (h *PipelineHandler) ClonePipeline(c *gin.Context) {
	p, err := h.pipelineSvc.GetPipeline(c.Request.Context(), c.Param("id"))
	if err != nil || p.TrainingConfig.Distillation == nil {
		c.JSON(400, gin.H{"code": 400, "message": "只能复用 2.0 任务配置"})
		return
	}
	var req cloneRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil && err != io.EOF {
		c.JSON(400, gin.H{"code": 400, "message": "复用配置格式无效，不能更改用户或所属项目"})
		return
	}
	copy, err := clonedConfiguration(p, req)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	copy.ID = uuid.NewString()
	if err := h.pipelineSvc.CreatePipeline(c.Request.Context(), copy); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	redactPipelineCredentials(copy)
	c.JSON(200, gin.H{"code": 200, "data": copy})
}

// DatasetTrainingConfig is the handoff consumed by the existing training page.
func (h *WorkflowHandler) DatasetTrainingConfig(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	d, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return
	}
	if d.SourceType != "generated" {
		c.JSON(422, gin.H{"code": 422, "message": "请选择蒸馏生成的训练数据"})
		return
	}
	recipe, ok := runtime.FindWorkflow(d.Lineage["job_type"])
	if !ok || recipe.OutputFormat != "sft_messages" && recipe.OutputFormat != "dpo_pairs" {
		c.JSON(422, gin.H{"code": 422, "message": "该产物不是 SFT/DPO 训练集"})
		return
	}
	stage := "sft"
	entry := map[string]interface{}{"file_name": d.FilePath, "formatting": "sharegpt", "columns": map[string]string{"messages": "messages"}, "tags": map[string]string{"role_tag": "role", "content_tag": "content", "user_tag": "user", "assistant_tag": "assistant", "system_tag": "system"}}
	if recipe.OutputFormat == "dpo_pairs" {
		stage = "dpo"
		entry = map[string]interface{}{"file_name": d.FilePath, "ranking": true, "columns": map[string]string{"prompt": "instruction", "query": "input", "chosen": "chosen", "rejected": "rejected"}}
	}
	if strings.HasPrefix(recipe.JobType, "mm_") {
		entry["columns"].(map[string]string)["images"] = "images"
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"dataset_id": d.ID, "format": recipe.OutputFormat, "stage": stage, "dataset": "gcs_distilled", "dataset_info": gin.H{"gcs_distilled": entry}, "requires_student_model": true, "creates_training_task": false}})
}
