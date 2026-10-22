package handlers

import (
	"net/http"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

type ResourceHandler struct {
	resources *service.ResourceService
	gcsClient *gcsclient.Client
}

func NewResourceHandler(resources *service.ResourceService, gcsClient *gcsclient.Client) *ResourceHandler {
	return &ResourceHandler{resources: resources, gcsClient: gcsClient}
}

func (h *ResourceHandler) Available(c *gin.Context) {
	out, err := h.resources.Available(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取可用资源成功",
		"data":    out,
	})
}

func (h *ResourceHandler) ListNodes(c *gin.Context) {
	nodes, err := h.gcsClient.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
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

func (h *ResourceHandler) Validate(c *gin.Context) {
	var request types.ResourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "资源选择格式无效"})
		return
	}
	if err := types.NormalizeResourceRequest(&request, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.resources.Validate(c.Request.Context(), &request, true, true); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"valid": true, "resource_request": request, "training_scope": "single_node_multi_gpu", "reserves_resources": false}})
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
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
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
