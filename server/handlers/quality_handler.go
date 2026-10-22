package handlers

import (
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

func (h *WorkflowHandler) Quality(c *gin.Context) {
	root, ok := h.artifact(c)
	if !ok {
		return
	}
	items, err := runtime.QualityReport(root)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "读取质量统计失败"})
		return
	}
	p, err := h.pipelines.GetPipeline(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Status(404)
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"summary": p.Summary, "scored_artifacts": items, "aggregation": "full_records_per_artifact", "model_evaluation": "not_performed", "token_usage": nil, "cost": nil}})
}
