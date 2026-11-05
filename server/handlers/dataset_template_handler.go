package handlers

import (
	"mime"
	"net/http"

	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

// ListSeedTemplates serves public format examples, never user-owned records.
func ListSeedTemplates(c *gin.Context) {
	job := c.Query("job_type")
	if job != "" {
		if _, ok := runtime.FindWorkflow(job); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "未知蒸馏配方"})
			return
		}
	}
	items, err := runtime.SeedTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取种子模板失败"})
		return
	}
	selected := []runtime.SeedTemplate{}
	for _, item := range items {
		for _, supported := range item.JobTypes {
			if job == "" || job == supported {
				selected = append(selected, item)
				break
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"items": selected, "formats": []string{"jsonl", "json"}, "encoding": "UTF-8",
		"examples_only": true, "bundle_download_path": "dataset-templates/download",
		"notice": "格式示例，不自动登记为业务数据；正式训练请替换为经过审核的业务种子。",
	}})
}

func DownloadSeedTemplate(c *gin.Context) {
	template, data, err := runtime.ReadSeedTemplate(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "种子模板不存在"})
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": template.Filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/x-ndjson; charset=utf-8", data)
}

func DownloadSeedTemplateBundle(c *gin.Context) {
	data, err := runtime.SeedTemplateBundle()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "打包种子模板失败"})
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "模型蒸馏种子模板.zip"}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/zip", data)
}
