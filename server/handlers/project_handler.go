package handlers

import (
	"net/http"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProjectHandler struct {
	projectSvc service.ProjectService
}

func NewProjectHandler(projectSvc service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectSvc: projectSvc}
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req types.Project
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "请求参数格式错误: " + err.Error()})
		return
	}

	req.ID = uuid.New().String()
	if err := h.projectSvc.CreateProject(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "项目创建成功", "data": req})
}

func (h *ProjectHandler) GetProject(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "项目ID不能为空"})
		return
	}

	project, err := h.projectSvc.GetProject(c.Request.Context(), uid, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "获取项目成功", "data": project})
}

func (h *ProjectHandler) ListProjects(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	projects, total, err := h.projectSvc.ListProjects(c.Request.Context(), uid, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "获取项目列表成功",
		"data": gin.H{
			"items":     projects,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "项目ID不能为空"})
		return
	}

	var req types.Project
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "请求参数格式错误: " + err.Error()})
		return
	}
	req.ID = id

	if err := h.projectSvc.UpdateProject(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "项目更新成功", "data": req})
}

func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "项目ID不能为空"})
		return
	}

	if err := h.projectSvc.DeleteProject(c.Request.Context(), uid, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "项目删除成功"})
}
