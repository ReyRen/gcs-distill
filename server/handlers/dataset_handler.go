package handlers

import (
	"net/http"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DatasetHandler struct {
	datasetSvc service.DatasetService
}

func NewDatasetHandler(datasetSvc service.DatasetService) *DatasetHandler {
	return &DatasetHandler{datasetSvc: datasetSvc}
}

func (h *DatasetHandler) CreateDataset(c *gin.Context) {
	var req types.Dataset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "请求参数格式错误: " + err.Error()})
		return
	}

	req.ID = uuid.New().String()
	if err := h.datasetSvc.CreateDataset(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "数据集创建成功", "data": req})
}

func (h *DatasetHandler) UploadDataset(c *gin.Context) {
	uid, ok := requireUIDForm(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "上传文件不能为空: " + err.Error()})
		return
	}
	uploadedFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": "打开上传文件失败: " + err.Error()})
		return
	}
	defer uploadedFile.Close()

	req := types.Dataset{
		ID:          uuid.New().String(),
		UID:         uid,
		Name:        c.PostForm("name"),
		Description: c.PostForm("description"),
		SourceType:  "upload",
	}
	if req.Name == "" {
		req.Name = fileHeader.Filename
	}

	if err := h.datasetSvc.CreateUploadedDataset(c.Request.Context(), &req, uploadedFile, fileHeader.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "数据集上传成功", "data": req})
}

func (h *DatasetHandler) GetDataset(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "数据集ID不能为空"})
		return
	}

	dataset, err := h.datasetSvc.GetDataset(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "获取数据集成功", "data": dataset})
}

func (h *DatasetHandler) ListDatasets(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	datasets, total, err := h.datasetSvc.ListDatasets(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
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

func (h *DatasetHandler) ListDatasetCandidates(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	candidates, err := h.datasetSvc.ListDatasetCandidates(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取数据集候选列表成功",
		"data": gin.H{
			"items": candidates,
			"total": len(candidates),
		},
	})
}

func (h *DatasetHandler) UpdateDataset(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "数据集ID不能为空"})
		return
	}

	var req types.Dataset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "请求参数格式错误: " + err.Error()})
		return
	}
	req.ID = id

	if err := h.datasetSvc.UpdateDataset(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "数据集更新成功", "data": req})
}

func (h *DatasetHandler) DeleteDataset(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "数据集ID不能为空"})
		return
	}

	if err := h.datasetSvc.DeleteDataset(c.Request.Context(), uid, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "数据集删除成功"})
}
