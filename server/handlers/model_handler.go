package handlers

import (
	"net/http"

	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

// ModelHandler 模型处理器
type ModelHandler struct {
	modelSvc service.ModelService
}

// NewModelHandler 创建模型处理器
func NewModelHandler(modelSvc service.ModelService) *ModelHandler {
	return &ModelHandler{
		modelSvc: modelSvc,
	}
}

// ListStudentModels 列出所有可用的学生模型
func (h *ModelHandler) ListStudentModels(c *gin.Context) {
	models, err := h.modelSvc.ListStudentModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取学生模型列表成功",
		"data":    models,
	})
}

// GetStudentModel 获取指定学生模型信息
func (h *ModelHandler) GetStudentModel(c *gin.Context) {
	modelID := c.Param("id")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "模型ID不能为空",
		})
		return
	}

	model, err := h.modelSvc.GetStudentModel(c.Request.Context(), modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取模型信息成功",
		"data":    model,
	})
}
