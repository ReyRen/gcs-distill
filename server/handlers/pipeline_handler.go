package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

// GetStageLogs 获取阶段完整日志
func (h *PipelineHandler) GetStageLogs(c *gin.Context) {
	stageID := c.Param("stage_id")
	if stageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "阶段ID不能为空",
		})
		return
	}

	// 获取阶段信息
	stage, err := h.pipelineSvc.GetStage(c.Request.Context(), stageID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段不存在",
		})
		return
	}

	// 检查日志路径
	if stage.LogPath == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "获取日志成功",
			"data": gin.H{
				"logs": "日志路径尚未设置，阶段可能还未开始执行",
			},
		})
		return
	}

	// 读取日志文件
	logContent, err := readLogFile(stage.LogPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "获取日志成功",
			"data": gin.H{
				"logs": fmt.Sprintf("无法读取日志文件: %v", err),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取日志成功",
		"data": gin.H{
			"logs":      logContent,
			"log_path":  stage.LogPath,
			"stage_id":  stage.ID,
			"stage_type": stage.StageType,
		},
	})
}

// StreamStageLogs 实时流式获取阶段日志
func (h *PipelineHandler) StreamStageLogs(c *gin.Context) {
	stageID := c.Param("stage_id")
	if stageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "阶段ID不能为空",
		})
		return
	}

	// 获取可选的tail参数
	tailLines := c.DefaultQuery("tail", "100")
	tail, _ := strconv.Atoi(tailLines)
	if tail <= 0 {
		tail = 100
	}

	// 获取阶段信息
	stage, err := h.pipelineSvc.GetStage(c.Request.Context(), stageID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段不存在",
		})
		return
	}

	// 检查日志路径
	if stage.LogPath == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "获取实时日志成功",
			"data": gin.H{
				"logs": "日志路径尚未设置，阶段可能还未开始执行",
			},
		})
		return
	}

	// 读取日志文件的最后N行
	logContent, err := readLogFileTail(stage.LogPath, tail)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"message": "获取实时日志成功",
			"data": gin.H{
				"logs": fmt.Sprintf("无法读取日志文件: %v", err),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取实时日志成功",
		"data": gin.H{
			"logs":       logContent,
			"log_path":   stage.LogPath,
			"stage_id":   stage.ID,
			"stage_type": stage.StageType,
			"status":     stage.Status,
		},
	})
}

// DownloadStageLogs 下载阶段日志文件
func (h *PipelineHandler) DownloadStageLogs(c *gin.Context) {
	stageID := c.Param("stage_id")
	if stageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "阶段ID不能为空",
		})
		return
	}

	// 获取阶段信息
	stage, err := h.pipelineSvc.GetStage(c.Request.Context(), stageID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段不存在",
		})
		return
	}

	// 检查日志路径
	if stage.LogPath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "日志文件不存在",
		})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(stage.LogPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "日志文件不存在",
		})
		return
	}

	// 设置下载文件名
	filename := fmt.Sprintf("stage_%s_%s.log", stage.StageType, stage.ID[:8])
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")

	// 发送文件
	c.File(stage.LogPath)
}

// readLogFile 读取日志文件完整内容
func readLogFile(logPath string) (string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("日志文件不存在")
	}

	// 读取文件内容
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	return string(content), nil
}

// readLogFileTail 读取日志文件的最后N行
func readLogFileTail(logPath string, lines int) (string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("日志文件不存在")
	}

	// 打开文件
	file, err := os.Open(logPath)
	if err != nil {
		return "", fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileSize := fileInfo.Size()

	// 如果文件很小，直接读取全部
	if fileSize < 8192 { // 8KB
		content, err := io.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("读取日志文件失败: %w", err)
		}
		return string(content), nil
	}

	// 否则读取最后的部分内容
	// 简化实现：读取最后64KB的内容（足够包含最后几百行）
	bufSize := int64(65536) // 64KB
	if bufSize > fileSize {
		bufSize = fileSize
	}

	buf := make([]byte, bufSize)
	_, err = file.ReadAt(buf, fileSize-bufSize)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	return string(buf), nil
}
