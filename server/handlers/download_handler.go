package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

func (h *WorkflowHandler) DownloadDataset(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	d, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return
	}
	root, err := h.storage.UserRoot(uid)
	if err != nil {
		c.Status(400)
		return
	}
	rel, err := filepath.Rel(root, d.FilePath)
	if err != nil {
		c.Status(404)
		return
	}
	path, err := runtime.SafeArtifactPath(root, rel)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "数据文件不可用"})
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}

func (h *WorkflowHandler) DownloadArtifact(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Set("download", "true")
	c.Request.URL.RawQuery = q.Encode()
	h.PreviewArtifact(c)
}

func (h *WorkflowHandler) DownloadModel(c *gin.Context) {
	root, ok := h.artifact(c)
	if !ok {
		return
	}
	items, err := runtime.ListArtifacts(root)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "读取模型产物失败"})
		return
	}
	files := []runtime.Artifact{}
	hasWeights := false
	for _, item := range items {
		if strings.Contains(strings.TrimPrefix(item.RelativePath, "models/checkpoints/"), "/") {
			continue
		}
		if strings.HasPrefix(item.Kind, "model_") {
			files = append(files, item)
		}
		if item.Kind == "model_weights" && item.Bytes > 0 {
			hasWeights = true
		}
	}
	if !hasWeights {
		c.JSON(404, gin.H{"code": 404, "message": "尚无学生模型权重"})
		return
	}
	// Check every file before sending headers; configuration and secrets are never included.
	for _, f := range files {
		if _, err := runtime.SafeArtifactPath(root, f.RelativePath); err != nil {
			c.Status(409)
			return
		}
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"student-%s.zip\"", c.Param("id")))
	w := zip.NewWriter(c.Writer)
	defer w.Close()
	for _, item := range files {
		path, err := runtime.SafeArtifactPath(root, item.RelativePath)
		if err != nil {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			return
		}
		entry, err := w.CreateHeader(&zip.FileHeader{Name: strings.TrimPrefix(item.RelativePath, "models/checkpoints/"), Method: zip.Store})
		if err != nil {
			f.Close()
			return
		}
		_, err = io.Copy(entry, f)
		f.Close()
		if err != nil {
			return
		}
	}
}
