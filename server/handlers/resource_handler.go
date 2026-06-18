package handlers

import (
	"net/http"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/gin-gonic/gin"
)

type ResourceHandler struct {
	gcsClient *gcsclient.Client
}

func NewResourceHandler(gcsClient *gcsclient.Client) *ResourceHandler {
	return &ResourceHandler{gcsClient: gcsClient}
}

func (h *ResourceHandler) ListNodes(c *gin.Context) {
	nodes, err := h.gcsClient.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取 gcs-v2 节点列表成功",
		"data":    nodes,
	})
}

func (h *ResourceHandler) GetNode(c *gin.Context) {
	nodeName := c.Param("name")
	if nodeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "节点名称不能为空",
		})
		return
	}

	node, found, err := h.gcsClient.GetNode(c.Request.Context(), nodeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "节点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取 gcs-v2 节点信息成功",
		"data":    node,
	})
}
