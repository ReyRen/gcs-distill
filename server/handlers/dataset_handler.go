package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DatasetHandler 数据集处理器
type DatasetHandler struct {
	datasetSvc service.DatasetService
}

// NewDatasetHandler 创建数据集处理器
func NewDatasetHandler(datasetSvc service.DatasetService) *DatasetHandler {
	return &DatasetHandler{
		datasetSvc: datasetSvc,
	}
}

// CreateDataset 创建数据集
func (h *DatasetHandler) CreateDataset(c *gin.Context) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		h.createUploadedDataset(c)
		return
	}

	var req types.Dataset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "请求参数格式错误: " + err.Error(),
		})
		return
	}

	// 生成 ID
	req.ID = uuid.New().String()

	// 创建数据集
	if err := h.datasetSvc.CreateDataset(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "数据集创建成功",
		"data":    req,
	})
}

func (h *DatasetHandler) createUploadedDataset(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		projectID = c.PostForm("project_id")
	}
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "项目ID不能为空",
		})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "上传文件不能为空: " + err.Error(),
		})
		return
	}

	uploadedFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "打开上传文件失败: " + err.Error(),
		})
		return
	}
	defer uploadedFile.Close()

	req := types.Dataset{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Name:        strings.TrimSpace(c.PostForm("name")),
		Description: strings.TrimSpace(c.PostForm("description")),
		SourceType:  "upload",
	}
	if req.Name == "" {
		req.Name = fileHeader.Filename
	}

	if err := h.datasetSvc.CreateUploadedDataset(c.Request.Context(), &req, uploadedFile, fileHeader.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "数据集上传成功",
		"data":    req,
	})
}

// GetDataset 获取数据集
func (h *DatasetHandler) GetDataset(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "数据集ID不能为空",
		})
		return
	}

	dataset, err := h.datasetSvc.GetDataset(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取数据集成功",
		"data":    dataset,
	})
}

// ListDatasets 列出数据集
func (h *DatasetHandler) ListDatasets(c *gin.Context) {
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

	datasets, total, err := h.datasetSvc.ListDatasets(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取数据集列表成功",
		"data": gin.H{
			"items":     datasets,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// UpdateDataset 更新数据集
func (h *DatasetHandler) UpdateDataset(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "数据集ID不能为空",
		})
		return
	}

	var req types.Dataset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "请求参数格式错误: " + err.Error(),
		})
		return
	}

	// 设置 ID
	req.ID = id

	// 更新数据集
	if err := h.datasetSvc.UpdateDataset(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "数据集更新成功",
		"data":    req,
	})
}

// DeleteDataset 删除数据集
func (h *DatasetHandler) DeleteDataset(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "数据集ID不能为空",
		})
		return
	}

	if err := h.datasetSvc.DeleteDataset(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "数据集删除成功",
	})
}
