package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
)

func SupportsDataExpansion(job string) bool {
	switch job {
	case "instruct_distill", "advanced_instruct_distill", "augmented_instruct_distill", "balanced_instruct_distill":
		return true
	}
	return false
}

func NormalizeDataExpansion(c *types.DataExpansionConfig, job string) error {
	if c == nil || !c.Enabled {
		return nil
	}
	if !SupportsDataExpansion(job) {
		return fmt.Errorf("此配方不支持种子指令扩充，请选择指令类配方")
	}
	if c.TargetRecords < 1 || c.TargetRecords > 100000 {
		return fmt.Errorf("目标产出条数必须为 1 到 100000")
	}
	if c.MaxCandidates == 0 {
		c.MaxCandidates = 3 * c.TargetRecords
	}
	if c.MaxCandidates < c.TargetRecords || c.MaxCandidates > 1000000 {
		return fmt.Errorf("候选生成预算须不小于目标条数，且不超过 1000000")
	}
	if c.BatchSize == 0 {
		c.BatchSize = min(max(c.TargetRecords, 4), 2000)
	}
	if c.BatchSize < 1 || c.BatchSize > 2000 {
		return fmt.Errorf("每轮候选条数上限必须为 1 到 2000")
	}
	if c.MaxRounds == 0 {
		c.MaxRounds = min(100, max(5, 3*((c.TargetRecords+c.BatchSize-1)/c.BatchSize)))
	}
	if c.MaxRounds < 1 || c.MaxRounds > 100 {
		return fmt.Errorf("补充生成轮数必须为 1 到 100")
	}
	if c.MaxRounds*c.BatchSize < c.TargetRecords {
		return fmt.Errorf("轮数与每轮候选条数上限不足以达到目标，请增加生成预算")
	}
	return nil
}

// Basic instruction mode gains the same official expansion operator used by
// augmented mode. Advanced recipes keep their own refinement/filtering stages.
func configureExpansion(cfg map[string]interface{}, c *types.DataExpansionConfig) error {
	if c == nil || !c.Enabled {
		if stages, ok := cfg["pipeline"].([]interface{}); ok {
			kept := []interface{}{}
			for _, raw := range stages {
				stage, _ := raw.(map[string]interface{})
				if stage["stage"] != "instruction_expansion" {
					kept = append(kept, raw)
				}
			}
			cfg["pipeline"] = kept
		}
		return nil
	}
	if err := NormalizeDataExpansion(c, fmt.Sprint(cfg["job_type"])); err != nil {
		return err
	}
	initial := nextExpansionBatch(*c, ExpansionProgress{})
	expansion := map[string]interface{}{"stage": "instruction_expansion", "config": map[string]interface{}{"num_in_context_samples": 2, "num_output_samples": initial, "temperature": 0.8, "max_tokens": 2048, "max_workers": 3}}
	if cfg["job_type"] == "instruct_distill" {
		cfg["job_type"] = "augmented_instruct_distill"
		cfg["pipeline"] = []interface{}{
			expansion,
			map[string]interface{}{"stage": "instruct_distill", "config": map[string]interface{}{}},
		}
	}
	stages, _ := cfg["pipeline"].([]interface{})
	found := 0
	for _, raw := range stages {
		stage, _ := raw.(map[string]interface{})
		if stage["stage"] != "instruction_expansion" {
			continue
		}
		found++
		config, _ := stage["config"].(map[string]interface{})
		if config == nil {
			config = map[string]interface{}{}
			stage["config"] = config
		}
		config["num_output_samples"] = initial
	}
	if found == 0 {
		cfg["pipeline"] = append([]interface{}{expansion}, stages...)
	} else if found != 1 {
		return fmt.Errorf("启用扩充时配方必须包含且仅包含一个 instruction_expansion 阶段")
	}
	return nil
}

type ExpansionRound struct {
	Round      int `json:"round"`
	Requested  int `json:"requested"`
	Usable     int `json:"usable_records"`
	Added      int `json:"added_records"`
	Duplicates int `json:"duplicate_records"`
}
type ExpansionProgress struct {
	CurrentRound int              `json:"current_round"`
	Target       int              `json:"target_records"`
	Actual       int              `json:"actual_records"`
	Candidates   int              `json:"candidates_requested"`
	Reached      bool             `json:"target_reached"`
	StopReason   string           `json:"stop_reason,omitempty"`
	Rounds       []ExpansionRound `json:"rounds"`
}

func nextExpansionBatch(c types.DataExpansionConfig, progress ExpansionProgress) int {
	remaining := c.TargetRecords - progress.Actual
	if remaining <= 0 {
		return 0
	}
	count := 2 * remaining
	if len(progress.Rounds) > 0 {
		prev := progress.Rounds[len(progress.Rounds)-1]
		if prev.Added > 0 {
			count = int(math.Ceil(float64(remaining) * float64(prev.Requested) / float64(prev.Added)))
		} else {
			count = c.BatchSize
		}
	}
	return min(count, c.BatchSize, c.MaxCandidates-progress.Candidates)
}

func expansionRoundConfig(base map[string]interface{}, root string, round, batch int) (map[string]interface{}, error) {
	b, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err = json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "data", "intermediate", fmt.Sprintf("expansion-%02d", round))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	cfg["dataset"].(map[string]interface{})["output_path"] = filepath.Join(dir, "dataset.jsonl")
	for idx, raw := range cfg["pipeline"].([]interface{}) {
		stage := raw.(map[string]interface{})
		stage["output_path"] = filepath.Join(dir, fmt.Sprintf("%02d_%s.jsonl", idx+1, stage["stage"]))
		if stage["stage"] == "instruction_expansion" {
			config := stage["config"].(map[string]interface{})
			config["num_output_samples"] = batch
			// A fixed user seed still produces new candidate contexts in later rounds.
			if seed, ok := config["seed"].(float64); ok {
				config["seed"] = int64(seed) + int64(round-1)*1000000
			}
		}
	}
	return cfg, nil
}

func validInstructionOutput(row map[string]interface{}) bool {
	user, assistant := false, false
	messages, _ := row["messages"].([]interface{})
	for _, raw := range messages {
		m, _ := raw.(map[string]interface{})
		v, ok := m["content"].(string)
		if !ok || normalizedText(v) == "" {
			continue
		}
		if m["role"] == "user" {
			user = true
		}
		if m["role"] == "assistant" {
			assistant = true
		}
	}
	return user && assistant
}

// collectExpansionRound deduplicates by prompt rather than answer so multiple
// answers to the same question cannot inflate the promised dataset size.
func collectExpansionRound(path string, out *json.Encoder, seen map[string]bool, limit int) (ExpansionRound, error) {
	r := ExpansionRound{}
	if info, err := os.Stat(path); err != nil {
		return r, err
	} else if info.Size() == 0 {
		return r, nil
	}
	_, err := WalkRecords(path, func(row map[string]interface{}) error {
		if !validInstructionOutput(row) {
			return nil
		}
		r.Usable++
		key := promptIdentity(row)
		if seen[key] {
			r.Duplicates++
			return nil
		}
		if r.Added >= limit {
			return nil
		}
		if err := out.Encode(row); err != nil {
			return err
		}
		seen[key] = true
		r.Added++
		return nil
	})
	if err != nil && err.Error() == "dataset contains no records" {
		err = nil
	}
	return r, err
}

func (e *StageExecutor) executeExpansion(ctx context.Context, stage *types.StageRun, project *types.Project, p *types.PipelineRun, base map[string]interface{}) error {
	c := *p.TrainingConfig.Distillation.DataExpansion
	root := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	aggregate := filepath.Join(root, "data", "intermediate", "expansion-collected.jsonl")
	f, err := os.OpenFile(aggregate, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	seen := map[string]bool{}
	progress := ExpansionProgress{Target: c.TargetRecords, Rounds: []ExpansionRound{}}
	logRoot := e.configGen.GetLogPath(p.UID, project.ID, p.ID, "distill_data")
	defer func() { stage.LogPath = logRoot }()
	persist := func() error {
		stage.Metrics = map[string]interface{}{"expansion": progress}
		stage.OutputManifest = map[string]string{"output_path": distill2OutputPath(root, p.TrainingConfig.Distillation.JobType), "record_count": fmt.Sprint(progress.Actual), "target_records": fmt.Sprint(c.TargetRecords), "target_reached": fmt.Sprint(progress.Reached), "expansion_manifest": "data/exported/expansion.json", "upstream_revision": EasyDistill2Revision}
		if err := writePrivateJSON(filepath.Join(root, "data", "exported", "expansion.json"), progress); err != nil {
			return err
		}
		if e.stageRepo != nil {
			return e.stageRepo.Update(ctx, stage)
		}
		return nil
	}
	for round := 1; round <= c.MaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := nextExpansionBatch(c, progress)
		if batch == 0 {
			break
		}
		cfg, err := expansionRoundConfig(base, root, round, batch)
		if err != nil {
			return err
		}
		path := filepath.Join(root, "configs", fmt.Sprintf("distill_round_%02d.json", round))
		if err := writePrivateJSON(path, cfg); err != nil {
			return err
		}
		stage.ConfigPath = path
		workers := p.ResourceRequest.ExecutorNodes
		if len(workers) == 0 {
			workers = e.dataWorkers
		}
		logPath := filepath.Join(logRoot, fmt.Sprintf("round-%02d", round))
		progress.CurrentRound = round
		progress.Candidates += batch
		if err := persist(); err != nil {
			return err
		}
		id, err := e.runContainerTask(ctx, &ContainerRequest{ContainerName: fmt.Sprintf("%s-r%d", stageContainerName(p.ID, stage.StageType), round), TaskUID: p.UID, Image: e.runtimeImage, Command: "gcs-easydistill", Args: []string{"--config", path}, HostWorkDir: root, ConfigPath: path, LogPath: logPath, ResourceMode: "cpu", WorkersName: workers, Mounts: []gcsclient.ContainerTaskMount{{Path: filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID)), ReadOnly: true}}})
		if err != nil {
			return err
		}
		if err := e.recordContainer(ctx, stage, id, logPath); err != nil {
			return err
		}
		err = e.waitForContainerTask(ctx, id, p.ID, stage)
		e.ReleaseContainer(stage)
		stage.LogPath = logRoot
		if logErr := archiveExpansionRound(logRoot, logPath, round); logErr != nil && err == nil {
			err = logErr
		}
		if err != nil {
			progress.StopReason = "container_failed"
			_ = persist()
			return err
		}
		stats, err := collectExpansionRound(cfg["dataset"].(map[string]interface{})["output_path"].(string), enc, seen, c.TargetRecords-progress.Actual)
		if err != nil {
			progress.StopReason = "invalid_round_output"
			_ = persist()
			return fmt.Errorf("扩充第 %d 轮产物无效: %w", round, err)
		}
		stats.Round = round
		stats.Requested = batch
		progress.Actual += stats.Added
		progress.Rounds = append(progress.Rounds, stats)
		progress.Reached = progress.Actual == c.TargetRecords
		if err := f.Sync(); err != nil {
			return err
		}
		if progress.Actual > 0 {
			if _, err := CopyRecords(aggregate, distill2OutputPath(root, p.TrainingConfig.Distillation.JobType), nil); err != nil {
				return err
			}
		}
		if progress.Reached {
			progress.StopReason = "target_reached"
		}
		if err := persist(); err != nil {
			return err
		}
		if progress.Reached {
			return nil
		}
	}
	progress.StopReason = "max_rounds"
	if progress.Candidates >= c.MaxCandidates {
		progress.StopReason = "candidate_budget"
	}
	if err := persist(); err != nil {
		return err
	}
	return fmt.Errorf("扩充未达目标：有效且去重后 %d/%d 条，已生成 %d 个候选、执行 %d 轮（%s）；已保留实际产物，不启动学生训练", progress.Actual, c.TargetRecords, progress.Candidates, len(progress.Rounds), progress.StopReason)
}

func archiveExpansionRound(root, roundPath string, round int) error {
	f, err := os.OpenFile(filepath.Join(root, "archived.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n=== Expansion round %d ===\n", round); err != nil {
		return err
	}
	for _, name := range []string{"log.txt", "archived.log"} {
		source, err := os.Open(filepath.Join(roundPath, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = io.Copy(f, source)
		source.Close()
		return err
	}
	return nil
}
