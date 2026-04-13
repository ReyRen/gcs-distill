package handlers

import (
	"net/http"

	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

// ResourceHandler 资源处理器
type ResourceHandler struct {
	schedulerSvc service.SchedulerService
}

// NewResourceHandler 创建资源处理器
func NewResourceHandler(schedulerSvc service.SchedulerService) *ResourceHandler {
	return &ResourceHandler{
		schedulerSvc: schedulerSvc,
	}
}

// ListNodes 列出所有节点
func (h *ResourceHandler) ListNodes(c *gin.Context) {
	nodes, err := h.schedulerSvc.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取节点列表成功",
		"data":    nodes,
	})
}

// GetNode 获取节点信息
func (h *ResourceHandler) GetNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "节点名称不能为空",
		})
		return
	}

	node, err := h.schedulerSvc.GetNode(c.Request.Context(), nodeName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取节点信息成功",
		"data":    node,
	})
}
