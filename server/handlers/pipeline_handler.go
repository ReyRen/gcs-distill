package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type PipelineHandler struct {
	pipelineSvc service.PipelineService
	gcsClient   *gcsclient.Client
	storage     config.StorageConfig
}

func NewPipelineHandler(pipelineSvc service.PipelineService, gcsClient *gcsclient.Client, storage config.StorageConfig) *PipelineHandler {
	return &PipelineHandler{
		pipelineSvc: pipelineSvc,
		gcsClient:   gcsClient,
		storage:     storage,
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

	redactPipelineCredentials(&req)
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

	redactPipelineCredentials(pipeline)
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取流水线成功",
		"data":    pipeline,
	})
}

func (h *PipelineHandler) ListPipelines(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	projectID := c.Query("project_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	pipelines, total, err := h.pipelineSvc.ListPipelines(c.Request.Context(), uid, projectID, page, pageSize, types.ListFilter{Name: c.Query("name"), Status: c.Query("status"), JobType: c.Query("job_type")})
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	for _, p := range pipelines {
		redactPipelineCredentials(p)
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
		c.JSON(http.StatusConflict, gin.H{
			"code":    http.StatusConflict,
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
		c.JSON(http.StatusConflict, gin.H{
			"code":    http.StatusConflict,
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
	logs, err := h.readStageLogs(c, stage, containerName, tail)
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
	if stage.Status == types.StatusSucceeded || stage.Status == types.StatusFailed || stage.Status == types.StatusCanceled {
		logs, err := h.readStageLogs(c, stage, containerName, tail)
		if err != nil {
			c.JSON(404, gin.H{"code": 404, "message": "日志暂不可用"})
			return
		}
		conn, err := stageLogUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, logs)
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "日志已结束"), time.Now().Add(time.Second))
		return
	}
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

	filename := fmt.Sprintf("stage_%s_%s.log", stage.StageType, shortID(stage.ID))
	if path, err := h.stageLogFile(c, stage); err == nil {
		c.FileAttachment(path, filename)
		return
	}
	logs, err := h.readStageLogs(c, stage, containerName, "all")
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Data(http.StatusOK, "application/octet-stream", logs)
}

func (h *PipelineHandler) stageLogFile(c *gin.Context, stage *types.StageRun) (string, error) {
	p, err := h.pipelineSvc.GetPipeline(c.Request.Context(), stage.PipelineRunID)
	if err != nil {
		return "", err
	}
	root := runtime.NewConfigGenerator(h.storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	for _, name := range []string{"log.txt", "archived.log"} {
		rel, err := filepath.Rel(root, filepath.Join(stage.LogPath, name))
		if err != nil {
			continue
		}
		path, err := runtime.SafeArtifactPath(root, rel)
		if err != nil {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func (h *PipelineHandler) readStageLogs(c *gin.Context, stage *types.StageRun, containerName, tail string) ([]byte, error) {
	if path, err := h.stageLogFile(c, stage); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if n, err := strconv.Atoi(tail); err == nil && n > 0 {
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if len(lines) > n {
				data = []byte(strings.Join(lines[len(lines)-n:], "\n"))
			}
		}
		return data, nil
	}
	return h.gcsClient.GetTaskLogs(c.Request.Context(), containerName, tail)
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
