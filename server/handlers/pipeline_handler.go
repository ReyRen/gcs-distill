package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type PipelineHandler struct {
	pipelineSvc service.PipelineService
	gcsClient   *gcsclient.Client
}

func NewPipelineHandler(pipelineSvc service.PipelineService, gcsClient *gcsclient.Client) *PipelineHandler {
	return &PipelineHandler{
		pipelineSvc: pipelineSvc,
		gcsClient:   gcsClient,
	}
}

func (h *PipelineHandler) CreatePipeline(c *gin.Context) {
	var req types.PipelineRun
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "请求参数格式错误: " + err.Error(),
		})
		return
	}

	req.ID = uuid.New().String()
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

func (h *PipelineHandler) ListPipelines(c *gin.Context) {
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

func (h *PipelineHandler) GetStageLogs(c *gin.Context) {
	stage, containerName, ok := h.stageLogTarget(c)
	if !ok {
		return
	}

	tail := c.DefaultQuery("tail", "100")
	logs, err := h.gcsClient.GetTaskLogs(c.Request.Context(), containerName, tail)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": err.Error(),
		})
		return
	}

	c.Header("X-GCS-Distill-Stage-ID", stage.ID)
	c.Header("X-GCS-Distill-Stage-Type", string(stage.StageType))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", logs)
}

func (h *PipelineHandler) StreamStageLogs(c *gin.Context) {
	h.GetStageLogs(c)
}

func (h *PipelineHandler) StreamStageLogsWebSocket(c *gin.Context) {
	stage, containerName, ok := h.stageLogTarget(c)
	if !ok {
		return
	}

	tail := c.DefaultQuery("tail", "100")
	targetURL, err := h.gcsClient.TaskLogsWebSocketURL(containerName, tail)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": err.Error(),
		})
		return
	}

	downstream, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		log.Printf("stage logs websocket dial failed pipeline_id=%s stage_id=%s container=%s target=%s error=%v",
			stage.PipelineRunID, stage.ID, containerName, targetURL, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": "连接 gcs-v2 日志 WebSocket 失败",
		})
		return
	}
	defer downstream.Close()

	upstream, err := stageLogUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("stage logs websocket upgrade failed pipeline_id=%s stage_id=%s error=%v", stage.PipelineRunID, stage.ID, err)
		return
	}
	defer upstream.Close()

	log.Printf("stage logs websocket proxy started pipeline_id=%s stage_id=%s container=%s target=%s",
		stage.PipelineRunID, stage.ID, containerName, targetURL)
	proxyStageLogWebSocket(upstream, downstream)
	log.Printf("stage logs websocket proxy ended pipeline_id=%s stage_id=%s container=%s", stage.PipelineRunID, stage.ID, containerName)
}

func (h *PipelineHandler) DownloadStageLogs(c *gin.Context) {
	stage, containerName, ok := h.stageLogTarget(c)
	if !ok {
		return
	}

	tail := c.DefaultQuery("tail", "10000")
	logs, err := h.gcsClient.GetTaskLogs(c.Request.Context(), containerName, tail)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": err.Error(),
		})
		return
	}

	filename := fmt.Sprintf("stage_%s_%s.log", stage.StageType, shortID(stage.ID))
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Data(http.StatusOK, "application/octet-stream", logs)
}

func (h *PipelineHandler) stageLogTarget(c *gin.Context) (*types.StageRun, string, bool) {
	if h.gcsClient == nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": "gcs-v2 client is not configured",
		})
		return nil, "", false
	}

	stageID := c.Param("stage_id")
	if stageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "阶段ID不能为空",
		})
		return nil, "", false
	}

	stage, err := h.pipelineSvc.GetStage(c.Request.Context(), stageID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段不存在",
		})
		return nil, "", false
	}

	if pipelineID := c.Param("id"); pipelineID != "" && stage.PipelineRunID != pipelineID {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段不属于当前流水线",
		})
		return nil, "", false
	}

	if stage.ContainerID == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "阶段容器尚未创建，日志暂不可用",
		})
		return nil, "", false
	}

	return stage, stage.ContainerID, true
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

var stageLogUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func proxyStageLogWebSocket(upstream, downstream *websocket.Conn) {
	done := make(chan struct{}, 2)
	go pumpStageLogWebSocket(done, downstream, upstream)
	go pumpStageLogWebSocket(done, upstream, downstream)
	<-done
}

func pumpStageLogWebSocket(done chan<- struct{}, dst, src *websocket.Conn) {
	defer func() { done <- struct{}{} }()
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			return
		}
	}
}
