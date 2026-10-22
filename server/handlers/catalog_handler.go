package handlers

import (
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

func (h *WorkflowHandler) SetCatalog(teachers *service.TeacherService, executor config.ExecutorConfig) {
	h.teachers = teachers
	h.executor = executor
}

func (h *WorkflowHandler) Teachers(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	items, err := h.teachers.List(c.Request.Context(), uid)
	if err != nil {
		c.JSON(502, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": items}})
}

func (h *WorkflowHandler) TestTeacher(c *gin.Context) {
	var req struct {
		UID     int               `json:"uid"`
		Teacher types.ModelConfig `json:"teacher_model"`
	}
	if c.ShouldBindJSON(&req) != nil || req.UID <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "uid 和 teacher_model 必填"})
		return
	}
	if err := h.teachers.Test(c.Request.Context(), req.UID, &req.Teacher); err != nil {
		c.JSON(422, gin.H{"code": 422, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"connected": true, "model_name": req.Teacher.ModelName, "checks": []string{"model_discovery", "chat_completion"}, "vision_verified": false}})
}

func (h *WorkflowHandler) RuntimeCatalog(c *gin.Context) {
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"architecture": "linux/amd64", "offline": true, "image_pull_policy": "never",
		"data_image": h.executor.RuntimeImage, "trainer_images": []string{h.executor.TrainerImage}, "data_workers": h.executor.DataWorkers,
		"finetuning_types": []string{"lora", "full"}, "model_templates": []string{"qwen", "llama3", "deepseek", "gemma", "mistral", "qwen2_vl"},
		"training_scope": "single_node_multi_gpu", "max_training_nodes": 1, "resource_selection": "explicit", "deferred_gpu_allocation": true, "weights_source": "user_private_model_pool", "model_evaluation": "separate_module"}})
}

// Scope all operations on a pipeline, including logs and WebSockets, to its UID.
func (h *PipelineHandler) AuthorizePipeline(c *gin.Context) {
	if c.Param("id") == "" {
		c.Next()
		return
	}
	uid, ok := requireUIDQuery(c)
	if !ok {
		c.Abort()
		return
	}
	p, err := h.pipelineSvc.GetPipeline(c.Request.Context(), c.Param("id"))
	if err != nil || p.UID != uid || c.Query("project_id") != "" && c.Query("project_id") != p.ProjectID {
		c.AbortWithStatusJSON(404, gin.H{"code": 404, "message": "任务不存在"})
		return
	}
	c.Next()
}
