package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
)

const MaxConsecutiveTeacherTimeouts = 3

// Counts are HTTP requests/operator outcomes, not final dataset rows. Advanced
// recipes perform several requests per sample. The teacher queue depth is unknown.
type GenerationProgress struct {
	ContainerID         string    `json:"container_id"`
	Phase               string    `json:"phase,omitempty"`
	SuccessfulRequests  int       `json:"successful_requests"`
	FailedRequests      int       `json:"failed_requests"`
	InvalidResults      int       `json:"invalid_results"`
	BuiltSamples        *int      `json:"built_samples"`
	RetryAttempts       int       `json:"retry_attempts"`
	TimeoutAttempts     int       `json:"timeout_attempts"`
	ConsecutiveTimeouts int       `json:"consecutive_timeout_attempts"`
	CircuitOpen         bool      `json:"circuit_open"`
	QueueDepth          *int      `json:"queue_depth"`
	UpdatedAt           time.Time `json:"updated_at"`
}

var nativePipelinePhase = regexp.MustCompile(`Running pipeline stage \d+/\d+: ([a-zA-Z0-9_]+)`)
var nativeBuiltSamples = regexp.MustCompile(`Built (\d+) SFT samples from \d+ generation results`)

type generationObserver struct {
	offset   int64
	progress GenerationProgress
}

func (o *generationObserver) consume(line string) {
	if strings.Contains(line, "Skipping invalid result for instruction:") {
		o.progress.InvalidResults++
	}
	if match := nativeBuiltSamples.FindStringSubmatch(line); len(match) > 1 {
		value, _ := strconv.Atoi(match[1])
		o.progress.BuiltSamples = &value
	}
	if match := nativePipelinePhase.FindStringSubmatch(line); len(match) > 1 {
		o.progress.Phase = match[1]
	}
	if strings.Contains(line, "POST ") && strings.Contains(line, "/chat/completions") && strings.Contains(line, "HTTP/1.1 200 OK") {
		o.progress.SuccessfulRequests++
		o.progress.ConsecutiveTimeouts = 0
	}
	retry := strings.Contains(line, "Generation failed for instruction ") && strings.Contains(line, "Retrying in ")
	failed := strings.Contains(line, "Failed to generate for instruction ") && strings.Contains(line, " attempts:")
	if retry {
		o.progress.RetryAttempts++
	}
	if failed {
		o.progress.FailedRequests++
	}
	if retry || failed {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout") {
			o.progress.TimeoutAttempts++
			o.progress.ConsecutiveTimeouts++
			if o.progress.ConsecutiveTimeouts >= MaxConsecutiveTeacherTimeouts {
				o.progress.CircuitOpen = true
			}
		} else {
			o.progress.ConsecutiveTimeouts = 0
		}
	}
}

// Consume complete new lines only, so polling never double counts a partial write.
func (o *generationObserver) read(path string) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() < o.offset {
		return false, fmt.Errorf("教师日志被截断，无法保证统计准确")
	}
	if _, err := f.Seek(o.offset, io.SeekStart); err != nil {
		return false, err
	}
	r := bufio.NewReader(f)
	changed := false
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return changed, err
		}
		o.offset += int64(len(line))
		o.consume(line)
		changed = true
	}
	if changed {
		o.progress.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func (e *StageExecutor) observeGeneration(ctx context.Context, stage *types.StageRun, o *generationObserver) error {
	changed, err := o.read(filepath.Join(stage.LogPath, "log.txt"))
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	o.progress.ContainerID = stage.ContainerID
	o.progress.CircuitOpen = o.progress.CircuitOpen || o.progress.ConsecutiveTimeouts >= MaxConsecutiveTeacherTimeouts
	if stage.Metrics == nil {
		stage.Metrics = map[string]interface{}{}
	}
	stage.Metrics["generation"] = o.progress
	if e.stageRepo != nil {
		if err := e.stageRepo.Update(ctx, stage); err != nil {
			return err
		}
	}
	if o.progress.CircuitOpen {
		return fmt.Errorf("已触发教师连续 %d 次调用超时保护，已停止本任务生成以避免持续排队重试；请降低并发或检查教师服务后重试。已落盘产物和日志保留", MaxConsecutiveTeacherTimeouts)
	}
	return nil
}
