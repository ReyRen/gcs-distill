package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

func (h *WorkflowHandler) splitDataset(c *gin.Context) (*types.Dataset, string, bool) {
	uid, ok := requireUIDQuery(c)
	if !ok {
		return nil, "", false
	}
	d, err := h.datasets.GetDataset(c.Request.Context(), uid, c.Param("id"))
	if err != nil || d.UID != uid {
		c.JSON(404, gin.H{"code": 404, "message": "数据集不存在"})
		return nil, "", false
	}
	recipe, ok := runtime.FindWorkflow(d.Lineage["job_type"])
	if d.SourceType != "generated" || !ok || (recipe.OutputFormat != "sft_messages" && recipe.OutputFormat != "dpo_pairs") {
		c.JSON(422, gin.H{"code": 422, "message": "请选择蒸馏生成的 SFT/DPO 数据集进行划分"})
		return nil, "", false
	}
	root, err := h.storage.UserDatasetGenerated(uid)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "无效的用户存储配置"})
		return nil, "", false
	}
	path, err := runtime.SafeArtifactPath(root, filepath.Join(d.ID, "dataset.jsonl"))
	if err != nil || path != d.FilePath {
		c.JSON(422, gin.H{"code": 422, "message": "数据集文件不在当前用户的生成数据目录中"})
		return nil, "", false
	}
	return d, filepath.Join(filepath.Dir(path), "splits"), true
}

func splitResponse(m *runtime.SplitManifest, uid int) gin.H {
	partitions := map[string]gin.H{}
	for name, p := range m.Partitions {
		part := gin.H{"records": p.Records, "groups": p.Groups, "actual_ratio": p.Ratio}
		if p.Records > 0 && !m.CreatedAt.IsZero() {
			base := fmt.Sprintf("datasets/%s/splits/%s/partitions/%s", m.DatasetID, m.ID, name)
			part["preview_path"] = fmt.Sprintf("%s/preview?uid=%d", base, uid)
			part["download_path"] = fmt.Sprintf("%s/download?uid=%d", base, uid)
		}
		partitions[name] = part
	}
	return gin.H{"id": m.ID, "dataset_id": m.DatasetID, "config": m.Config, "source_sha256": m.SourceSHA256, "total_records": m.TotalRecords, "total_groups": m.TotalGroups, "partitions": partitions, "warnings": m.Warnings, "created_at": m.CreatedAt}
}

func (h *WorkflowHandler) PreviewDatasetSplit(c *gin.Context) { h.writeDatasetSplit(c, false) }
func (h *WorkflowHandler) CreateDatasetSplit(c *gin.Context)  { h.writeDatasetSplit(c, true) }
func (h *WorkflowHandler) writeDatasetSplit(c *gin.Context, create bool) {
	d, base, ok := h.splitDataset(c)
	if !ok {
		return
	}
	var cfg types.DataSplitConfig
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "划分配置格式无效"})
		return
	}
	if err := runtime.NormalizeDataSplit(&cfg); err != nil || !cfg.Enabled {
		message := "请启用数据划分"
		if err != nil {
			message = err.Error()
		}
		c.JSON(400, gin.H{"code": 400, "message": message})
		return
	}
	var m *runtime.SplitManifest
	var err error
	if create {
		m, err = runtime.CreateDataSplit(d.FilePath, base, d.ID, cfg)
	} else {
		m, err = runtime.PreviewDataSplit(d.FilePath, d.ID, cfg)
	}
	if err != nil {
		c.JSON(422, gin.H{"code": 422, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": splitResponse(m, d.UID)})
}

func (h *WorkflowHandler) ListDatasetSplits(c *gin.Context) {
	d, base, ok := h.splitDataset(c)
	if !ok {
		return
	}
	files, err := os.ReadDir(base)
	if err != nil && !os.IsNotExist(err) {
		c.JSON(500, gin.H{"code": 500, "message": "读取划分记录失败"})
		return
	}
	manifests := []*runtime.SplitManifest{}
	for _, f := range files {
		if !f.IsDir() || len(f.Name()) != 32 {
			continue
		}
		m, err := runtime.ReadDataSplit(base, f.Name())
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "划分清单损坏"})
			return
		}
		manifests = append(manifests, m)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].CreatedAt.After(manifests[j].CreatedAt) })
	items := []gin.H{}
	for _, m := range manifests {
		items = append(items, splitResponse(m, d.UID))
	}
	attached, err := runtime.AttachedDataSplit(filepath.Dir(d.FilePath))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "读取默认划分失败"})
		return
	}
	defaultID := ""
	if attached != nil {
		defaultID = attached.ID
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": items, "default_split_id": defaultID}})
}

func (h *WorkflowHandler) GetDatasetSplit(c *gin.Context) {
	d, base, ok := h.splitDataset(c)
	if !ok {
		return
	}
	m, err := runtime.ReadDataSplit(base, c.Param("split_id"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "划分不存在"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": splitResponse(m, d.UID)})
}

func (h *WorkflowHandler) PreviewSplitPartition(c *gin.Context)  { h.readSplitPartition(c, false) }
func (h *WorkflowHandler) DownloadSplitPartition(c *gin.Context) { h.readSplitPartition(c, true) }
func (h *WorkflowHandler) readSplitPartition(c *gin.Context, download bool) {
	_, base, ok := h.splitDataset(c)
	if !ok {
		return
	}
	m, err := runtime.ReadDataSplit(base, c.Param("split_id"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "划分不存在"})
		return
	}
	part, ok := m.Partitions[c.Param("partition")]
	if !ok || part.Records == 0 {
		c.JSON(404, gin.H{"code": 404, "message": "此划分集合不存在或未启用"})
		return
	}
	path, err := runtime.SafeArtifactPath(base, filepath.Join(m.ID, c.Param("partition")+".jsonl"))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "划分文件不存在"})
		return
	}
	if download {
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
		c.FileAttachment(path, c.Param("partition")+".jsonl")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 || limit > 100 {
		limit = 10
	}
	rows, total, fields, err := runtime.PreviewRecords(path, limit)
	if err != nil {
		c.JSON(422, gin.H{"code": 422, "message": "划分文件无效"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"items": rows, "total": total, "fields": fields, "limit": limit, "partition": c.Param("partition")}})
}
