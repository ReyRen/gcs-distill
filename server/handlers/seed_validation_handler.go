package handlers

import (
	"encoding/json"
	"io"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

// ValidateDatasetSeed runs before teacher/resource selection. It never calls a
// model, creates a task, or rewrites the source dataset.
func (h *WorkflowHandler) ValidateDatasetSeed(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	d, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil || d.UID != uid {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return
	}
	var request struct {
		JobType      string            `json:"job_type"`
		SeedLimit    int               `json:"seed_limit"`
		FieldMapping map[string]string `json:"field_mapping"`
	}
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 65537))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "种子校验参数格式无效"})
		return
	}
	if _, ok := runtime.FindWorkflow(request.JobType); !ok {
		c.JSON(400, gin.H{"code": 400, "message": "请选择有效的任务类型"})
		return
	}
	w := &types.DistillationConfig{JobType: request.JobType, SeedLimit: request.SeedLimit, FieldMapping: request.FieldMapping}
	selection, err := runtime.InspectSeedSelection(c.Request.Context(), d.FilePath, w)
	if err != nil {
		c.JSON(422, gin.H{"code": 422, "message": err.Error(), "data": gin.H{"valid": false, "job_type": request.JobType}})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"valid": true, "dataset_id": d.ID, "job_type": request.JobType, "seed_selection": selection, "checks": []string{"complete_json_syntax", "selected_record_schema"}, "teacher_checked": false, "resources_reserved": false}})
}

func (h *WorkflowHandler) DatasetCompatibleWorkflows(c *gin.Context) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return
	}
	d, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil || d.UID != uid {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("seed_limit", "0"))
	if err != nil || limit < 0 {
		c.JSON(400, gin.H{"code": 400, "message": "seed_limit 必须是非负整数"})
		return
	}
	result, err := runtime.InspectWorkflowCompatibility(c.Request.Context(), d.FilePath, limit)
	if err != nil {
		c.JSON(422, gin.H{"code": 422, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": result})
}
