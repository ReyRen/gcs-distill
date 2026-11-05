package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
)

var splitNames = []string{"train", "validation", "test"}

type SplitPartition struct {
	Records int     `json:"records"`
	Groups  int     `json:"groups"`
	Ratio   float64 `json:"actual_ratio"`
	File    string  `json:"file,omitempty"`
}

type SplitManifest struct {
	ID           string                    `json:"id"`
	DatasetID    string                    `json:"dataset_id"`
	Config       types.DataSplitConfig     `json:"config"`
	SourceSHA256 string                    `json:"source_sha256"`
	TotalRecords int                       `json:"total_records"`
	TotalGroups  int                       `json:"total_groups"`
	Partitions   map[string]SplitPartition `json:"partitions"`
	Warnings     []string                  `json:"warnings"`
	CreatedAt    time.Time                 `json:"created_at,omitempty"`
}

func NormalizeDataSplit(c *types.DataSplitConfig) error {
	if c == nil || !c.Enabled {
		return nil
	}
	values := []float64{c.TrainRatio, c.ValidationRatio, c.TestRatio}
	sum := 0.0
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return fmt.Errorf("划分比例必须在 0 到 1 之间")
		}
		sum += v
	}
	if math.Abs(sum-1) > 1e-8 || c.TrainRatio <= 0 || c.TrainRatio == 1 {
		return fmt.Errorf("划分比例之和必须为 1，训练集比例须大于 0，且至少保留验证集或测试集")
	}
	if c.GroupBy == "" {
		c.GroupBy = "auto"
	}
	if len(c.GroupBy) > 160 {
		return fmt.Errorf("分组字段路径过长")
	}
	for _, part := range strings.Split(c.GroupBy, ".") {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("分组字段路径无效")
		}
	}
	return nil
}

func digest(data []byte) string      { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func normalizedText(v string) string { return strings.ToLower(strings.Join(strings.Fields(v), " ")) }

func fieldValue(row map[string]interface{}, path string) interface{} {
	var value interface{} = row
	for _, name := range strings.Split(path, ".") {
		m, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		value = m[name]
	}
	return value
}

// promptIdentity deliberately excludes answers, scores and run-level provenance.
func promptIdentity(row map[string]interface{}) string {
	var parts []string
	if messages, ok := row["messages"].([]interface{}); ok {
		for _, raw := range messages {
			m, _ := raw.(map[string]interface{})
			if m["role"] != "user" {
				continue
			}
			if content, ok := m["content"].(string); ok {
				parts = append(parts, normalizedText(content))
			} else {
				content, _ := json.Marshal(m["content"])
				parts = append(parts, string(content))
			}
			break
		}
	}
	if len(parts) == 0 {
		for _, key := range []string{"instruction", "problem", "prompt", "input"} {
			if v, ok := row[key].(string); ok && v != "" {
				parts = append(parts, v)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if images, ok := row["images"]; ok {
		b, _ := json.Marshal(images)
		parts = append(parts, string(b))
	}
	return digest([]byte(normalizedText(strings.Join(parts, "\n"))))
}

func groupingKeys(row map[string]interface{}, cfg types.DataSplitConfig) ([]string, bool, error) {
	keys := []string{}
	explicit := false
	if cfg.GroupBy != "auto" {
		value := fieldValue(row, cfg.GroupBy)
		if value == nil || value == "" {
			return nil, false, fmt.Errorf("记录缺少分组字段 %s", cfg.GroupBy)
		}
		switch value.(type) {
		case string, float64, json.Number:
		default:
			return nil, false, fmt.Errorf("分组字段 %s 必须是字符串或数值", cfg.GroupBy)
		}
		b, _ := json.Marshal(value)
		keys = append(keys, "custom:"+string(b))
		explicit = true
	} else {
		for _, name := range []string{"seed_id", "source_id", "conversation_id", "parent_id"} {
			for _, path := range []string{name, "metadata." + name} {
				value := fieldValue(row, path)
				if value == nil || value == "" {
					continue
				}
				switch value.(type) {
				case string, float64, json.Number:
				default:
					continue
				}
				b, _ := json.Marshal(value)
				keys = append(keys, name+":"+string(b))
				explicit = true
			}
		}
	}
	if key := promptIdentity(row); key != "" {
		keys = append(keys, "prompt:"+key)
	}
	// Shared images must never cross partitions, even with different questions.
	if images, ok := row["images"].([]interface{}); ok {
		for _, image := range images {
			b, _ := json.Marshal(image)
			keys = append(keys, "image:"+digest(b))
		}
	}
	if len(keys) == 0 {
		return nil, false, fmt.Errorf("记录没有可用于防泄漏分组的问题、图片或来源标识")
	}
	return keys, explicit, nil
}

type splitPlan struct {
	manifest    SplitManifest
	assignments []int
}

func planSplit(path, datasetID string, cfg types.DataSplitConfig) (*splitPlan, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("请启用数据划分")
	}
	if err := NormalizeDataSplit(&cfg); err != nil {
		return nil, err
	}
	parents, identities := []int{}, []string{}
	owners := map[string]int{}
	var find func(int) int
	find = func(i int) int {
		if parents[i] != i {
			parents[i] = find(parents[i])
		}
		return parents[i]
	}
	hash := sha256.New()
	missingSource := 0
	total, err := WalkRecords(path, func(row map[string]interface{}) error {
		keys, explicit, err := groupingKeys(row, cfg)
		if err != nil {
			return err
		}
		if !explicit {
			missingSource++
		}
		b, err := json.Marshal(row)
		if err != nil {
			return err
		}
		hash.Write(b)
		hash.Write([]byte{'\n'})
		i := len(parents)
		parents = append(parents, i)
		sort.Strings(keys)
		identities = append(identities, keys[0])
		for _, key := range keys {
			if prev, ok := owners[key]; ok {
				parents[find(i)] = find(prev)
			} else {
				owners[key] = i
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	type group struct {
		members         []int
		identity, order string
	}
	groups := map[int]*group{}
	for i := range parents {
		root := find(i)
		g := groups[root]
		if g == nil {
			g = &group{identity: identities[i]}
			groups[root] = g
		}
		g.members = append(g.members, i)
		if identities[i] < g.identity {
			g.identity = identities[i]
		}
	}
	ratios := []float64{cfg.TrainRatio, cfg.ValidationRatio, cfg.TestRatio}
	active := []int{}
	for i, ratio := range ratios {
		if ratio > 0 {
			active = append(active, i)
		}
	}
	if len(groups) < len(active) {
		return nil, fmt.Errorf("只有 %d 个独立样本组，无法划分为 %d 个非空集合；请增加样本或减少集合数量", len(groups), len(active))
	}
	ordered := []*group{}
	for _, g := range groups {
		g.order = digest([]byte(fmt.Sprintf("%d:%s", cfg.Seed, g.identity)))
		ordered = append(ordered, g)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i].members) != len(ordered[j].members) {
			return len(ordered[i].members) > len(ordered[j].members)
		}
		return ordered[i].order < ordered[j].order
	})
	counts, groupCounts := [3]int{}, [3]int{}
	assignments := make([]int, total)
	for idx, g := range ordered {
		selected := -1
		remainingGroups := len(ordered) - idx
		empty := []int{}
		for _, i := range active {
			if counts[i] == 0 {
				empty = append(empty, i)
			}
		}
		if remainingGroups == len(empty) {
			selected = empty[0]
		} else {
			best := math.Inf(-1)
			for _, i := range active {
				deficit := ratios[i]*float64(total) - float64(counts[i])
				if deficit > best {
					best = deficit
					selected = i
				}
			}
		}
		counts[selected] += len(g.members)
		groupCounts[selected]++
		for _, i := range g.members {
			assignments[i] = selected
		}
	}
	m := SplitManifest{DatasetID: datasetID, Config: cfg, SourceSHA256: hex.EncodeToString(hash.Sum(nil)), TotalRecords: total, TotalGroups: len(groups), Partitions: map[string]SplitPartition{}, Warnings: []string{}}
	if missingSource > 0 {
		m.Warnings = append(m.Warnings, fmt.Sprintf("%d 条记录没有种子/会话来源标识，使用问题和图片分组；无法追溯其原始种子关系", missingSource))
	}
	ratioWarning := false
	for i, name := range splitNames {
		part := SplitPartition{Records: counts[i], Groups: groupCounts[i], Ratio: float64(counts[i]) / float64(total)}
		if ratios[i] > 0 {
			part.File = name + ".jsonl"
		}
		m.Partitions[name] = part
		if math.Abs(part.Ratio-ratios[i]) > 0.05 {
			ratioWarning = true
		}
	}
	if ratioWarning {
		m.Warnings = append(m.Warnings, "分组完整性优先于精确比例，请核对实际划分条数")
	}
	b, _ := json.Marshal(struct {
		DatasetID, Hash string
		Config          types.DataSplitConfig
	}{datasetID, m.SourceSHA256, cfg})
	m.ID = digest(b)[:32]
	return &splitPlan{manifest: m, assignments: assignments}, nil
}

func PreviewDataSplit(path, datasetID string, cfg types.DataSplitConfig) (*SplitManifest, error) {
	p, err := planSplit(path, datasetID, cfg)
	if err != nil {
		return nil, err
	}
	return &p.manifest, nil
}

// CreateDataSplit publishes an immutable snapshot atomically. Identical input and
// settings reuse the same snapshot; preview and creation share the exact planner.
func CreateDataSplit(path, base, datasetID string, cfg types.DataSplitConfig) (*SplitManifest, error) {
	p, err := planSplit(path, datasetID, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := EnsureStorageDir(filepath.Dir(base), filepath.Base(base)); err != nil {
		return nil, err
	}
	target := filepath.Join(base, p.manifest.ID)
	if m, err := ReadDataSplit(base, p.manifest.ID); err == nil {
		return m, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	tmp, err := os.MkdirTemp(base, ".split-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	files := make([]*os.File, 3)
	encoders := make([]*json.Encoder, 3)
	defer func() {
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
	}()
	for i, name := range splitNames {
		if p.manifest.Partitions[name].Records == 0 {
			continue
		}
		f, err := os.OpenFile(filepath.Join(tmp, name+".jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		files[i] = f
		encoders[i] = json.NewEncoder(f)
	}
	i := 0
	hash := sha256.New()
	_, err = WalkRecords(path, func(row map[string]interface{}) error {
		if i >= len(p.assignments) {
			return fmt.Errorf("划分时源数据发生变化")
		}
		b, _ := json.Marshal(row)
		hash.Write(b)
		hash.Write([]byte{'\n'})
		err := encoders[p.assignments[i]].Encode(row)
		i++
		return err
	})
	if err != nil {
		return nil, err
	}
	if i != len(p.assignments) || hex.EncodeToString(hash.Sum(nil)) != p.manifest.SourceSHA256 {
		return nil, fmt.Errorf("划分时源数据发生变化，请重试")
	}
	for _, f := range files {
		if f != nil {
			if err := f.Close(); err != nil {
				return nil, err
			}
		}
	}
	p.manifest.CreatedAt = time.Now().UTC()
	if err := writePrivateJSON(filepath.Join(tmp, "manifest.json"), p.manifest); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, target); err != nil {
		if m, e := ReadDataSplit(base, p.manifest.ID); e == nil {
			return m, nil
		}
		return nil, err
	}
	return &p.manifest, nil
}

func ReadDataSplit(base, id string) (*SplitManifest, error) {
	if len(id) != 32 {
		return nil, fmt.Errorf("无效的划分 ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, fmt.Errorf("无效的划分 ID")
	}
	path, err := SafeArtifactPath(base, filepath.Join(id, "manifest.json"))
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m SplitManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.ID != id {
		return nil, fmt.Errorf("划分清单 ID 不匹配")
	}
	return &m, nil
}

// AttachedDataSplit returns the export-time default; later manual snapshots do
// not silently replace the dataset's original training configuration.
func AttachedDataSplit(dir string) (*SplitManifest, error) {
	path, err := SafeArtifactPath(dir, "manifest.json")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m["split_id"] == "" {
		return nil, nil
	}
	return ReadDataSplit(filepath.Join(dir, "splits"), m["split_id"])
}

func TrainingSplitEntries(entry map[string]interface{}, base string, m *SplitManifest) map[string]interface{} {
	entries := map[string]interface{}{}
	for _, name := range []string{"train", "validation"} {
		if m.Partitions[name].Records == 0 {
			continue
		}
		copy := map[string]interface{}{}
		for k, v := range entry {
			copy[k] = v
		}
		copy["file_name"] = filepath.Join(base, m.ID, name+".jsonl")
		key := "gcs_distilled"
		if name == "validation" {
			key = "gcs_validation"
		}
		entries[key] = copy
	}
	return entries
}
