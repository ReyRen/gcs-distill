package handlers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	pipelines service.PipelineService
	projects  service.ProjectService
	datasets  service.DatasetService
	storage   config.StorageConfig
	teachers  *service.TeacherService
	executor  config.ExecutorConfig
}

func NewWorkflowHandler(pipelines service.PipelineService, projects service.ProjectService, datasets service.DatasetService, storage config.StorageConfig) *WorkflowHandler {
	return &WorkflowHandler{pipelines: pipelines, projects: projects, datasets: datasets, storage: storage}
}

func (h *WorkflowHandler) PreviewDataset(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	dataset, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	rows, total, fields, err := runtime.PreviewRecords(dataset.FilePath, limit)
	if err != nil {
		c.JSON(422, gin.H{"code": 422, "message": err.Error()})
		return
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": rows, "total": total, "fields": fields, "limit": limit}})
}

func (h *WorkflowHandler) Preview(c *gin.Context) {
	var p types.PipelineRun
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "配置格式无效"})
		return
	}
	project, err := h.projects.GetProject(c.Request.Context(), p.UID, p.ProjectID)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "项目不存在"})
		return
	}
	if err := h.pipelines.ValidatePipeline(c.Request.Context(), &p); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	p.ID = "preview"
	cfg, err := runtime.NewConfigGenerator(h.storage.RootPath).GenerateDistill2Config(project, &p)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	redactConfig(cfg)
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"config": cfg, "stages": runtime.PipelineStageTypes(&p), "resource_mode": "cpu"}})
}

func redactConfig(value interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if key == "api_key" || key == "token" || key == "password" {
				v[key] = "********"
			} else {
				redactConfig(child)
			}
		}
	case []interface{}:
		for _, child := range v {
			redactConfig(child)
		}
	}
}

func (h *WorkflowHandler) Summary(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	if _, err := h.projects.GetProject(c.Request.Context(), uid, c.Param("id")); err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "项目不存在"})
		return
	}
	counts := map[string]int{}
	total := 0
	for page := 1; ; page++ {
		items, n, err := h.pipelines.ListPipelines(c.Request.Context(), uid, c.Param("id"), page, 100)
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "读取项目统计失败"})
			return
		}
		total = n
		for _, p := range items {
			counts[string(p.Status)]++
		}
		if len(items) < 100 || page*100 >= total {
			break
		}
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"total": total, "status_counts": counts}})
}

func (h *WorkflowHandler) List(c *gin.Context) {
	items := []gin.H{}
	for _, recipe := range runtime.WorkflowRecipes() {
		defaults, err := runtime.WorkflowDefaults(recipe)
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "读取配方失败"})
			return
		}
		redactConfig(defaults)
		items = append(items, gin.H{"job_type": recipe.JobType, "name": recipe.Name, "category": recipe.Category, "output_format": recipe.OutputFormat, "input_fields": recipe.InputFields, "default_parameters": defaults})
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"engine": "easydistill", "version": "2.0.0", "upstream_revision": runtime.EasyDistill2Revision,
		"runtime_image": runtime.EasyDistill2Image, "items": items, "default_job_type": "instruct_distill",
		"data_resource_mode": "cpu", "student_training": "optional_llamafactory", "model_evaluation": "separate_workflow"}})
}

func (h *WorkflowHandler) artifact(c *gin.Context) (string, bool) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return "", false
	}
	p, err := h.pipelines.GetPipeline(c.Request.Context(), c.Param("id"))
	if err != nil || p.UID != uid {
		c.JSON(404, gin.H{"code": 404, "message": "流水线不存在"})
		return "", false
	}
	root := runtime.NewConfigGenerator(h.storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	return root, true
}

func (h *WorkflowHandler) Artifacts(c *gin.Context) {
	root, ok := h.artifact(c)
	if !ok {
		return
	}
	items, err := runtime.ListArtifacts(root)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "读取产物失败"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": items}})
}

func (h *WorkflowHandler) PreviewArtifact(c *gin.Context) {
	root, ok := h.artifact(c)
	if !ok {
		return
	}
	items, err := runtime.ListArtifacts(root)
	if err != nil {
		c.JSON(500, gin.H{"code": 500})
		return
	}
	for _, item := range items {
		if item.RelativePath != c.Query("path") {
			continue
		}
		path, err := runtime.SafeArtifactPath(root, item.RelativePath)
		if err != nil {
			break
		}
		if c.Query("download") == "true" {
			c.FileAttachment(path, item.Name)
			return
		}
		if !item.Previewable {
			c.JSON(415, gin.H{"code": 415, "message": "此产物只支持下载"})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		if limit < 1 || limit > 100 {
			limit = 10
		}
		if item.Name == "manifest.json" {
			data, err := os.ReadFile(path)
			if err != nil {
				break
			}
			c.Data(200, "application/json", data)
			return
		}
		rows, total, fields, err := runtime.PreviewRecords(path, limit)
		if err != nil {
			c.JSON(422, gin.H{"code": 422, "message": "产物格式无效"})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": rows, "total": total, "limit": limit, "fields": fields}})
		return
	}
	c.JSON(404, gin.H{"code": 404, "message": "产物不存在"})
}
