package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PipelineHandler 流水线处理器
type PipelineHandler struct {
	pipelineSvc service.PipelineService
}

// NewPipelineHandler 创建流水线处理器
func NewPipelineHandler(pipelineSvc service.PipelineService) *PipelineHandler {
	return &PipelineHandler{
		pipelineSvc: pipelineSvc,
	}
}

// CreatePipeline 创建流水线
func (h *PipelineHandler) CreatePipeline(c *gin.Context) {
	var req types.PipelineRun
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "请求参数格式错误: " + err.Error(),
		})
		return
	}

	// 生成 ID
	req.ID = uuid.New().String()

	// 创建流水线
	if err := h.pipelineSvc.CreatePipeline(c.Request.Context(), &req); err != nil {
		_ = c.Error(err)

		statusCode := http.StatusInternalServerError
		var validationErr *service.ValidationError
		if errors.As(err, &validationErr) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"code":    statusCode,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "流水线创建成功",
		"data":    req,
	})
}

// GetPipeline 获取流水线
func (h *PipelineHandler) GetPipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "流水线ID不能为空",
		})
		return
	}

	pipeline, err := h.pipelineSvc.GetPipeline(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取流水线成功",
		"data":    pipeline,
	})
}

// ListPipelines 列出流水线
func (h *PipelineHandler) ListPipelines(c *gin.Context) {
	// 解析查询参数
	projectID := c.Query("project_id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "项目ID不能为空",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	pipelines, total, err := h.pipelineSvc.ListPipelines(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取流水线列表成功",
		"data": gin.H{
			"items":     pipelines,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// StartPipeline 启动流水线
func (h *PipelineHandler) StartPipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "流水线ID不能为空",
		})
		return
	}

	if err := h.pipelineSvc.StartPipeline(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "流水线启动成功",
	})
}

// CancelPipeline 取消流水线
func (h *PipelineHandler) CancelPipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "流水线ID不能为空",
		})
		return
	}

	if err := h.pipelineSvc.CancelPipeline(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "流水线取消成功",
	})
}

// ListStages 列出流水线的所有阶段
func (h *PipelineHandler) ListStages(c *gin.Context) {
	pipelineID := c.Param("id")
	if pipelineID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "流水线ID不能为空",
		})
		return
	}

	stages, err := h.pipelineSvc.ListStages(c.Request.Context(), pipelineID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取阶段列表成功",
		"data":    stages,
	})
}
