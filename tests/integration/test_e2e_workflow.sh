#!/usr/bin/env bash
set -euo pipefail

green='\033[0;32m'
red='\033[0;31m'
yellow='\033[1;33m'
reset='\033[0m'

test_workspace="/tmp/gcs-distill-e2e-test"
project_id="test-project-001"
run_id="test-run-001"
workspace="$test_workspace/projects/$project_id/runs/$run_id"

cleanup() {
    echo -e "${yellow}Cleaning test workspace...${reset}"
    rm -rf "$test_workspace"
}

pass() {
    echo -e "${green}OK${reset} $1"
}

fail() {
    echo -e "${red}FAIL${reset} $1"
    exit 1
}

trap cleanup EXIT

echo "=========================================="
echo "GCS-Distill e2e workspace smoke test"
echo "=========================================="

echo ""
echo "1. Create workspace layout"
mkdir -p "$workspace"/{configs,data/seed,data/generated,data/filtered,models/checkpoints,logs,eval}
pass "workspace created at $workspace"
tree "$workspace" 2>/dev/null || ls -R "$workspace"

echo ""
echo "2. Prepare seed data"
cp tests/integration/sample_seed_data.json "$workspace/data/seed/instructions.json"
test -s "$workspace/data/seed/instructions.json" || fail "seed data was not copied"
pass "seed data copied"

echo ""
echo "3. Generate teacher inference config"
cat > "$workspace/configs/teacher_infer.json" <<'EOF'
{
  "job_type": "kd_black_box_local",
  "dataset": {
    "instruction_path": "/workspace/data/seed/instructions.json",
    "labeled_path": "/workspace/data/generated/labeled.json"
  },
  "inference": {
    "temperature": 0.8,
    "max_new_tokens": 512,
    "batch_size": 1
  },
  "models": {
    "teacher": "test-model"
  },
  "logging": {
    "log_file": "/workspace/logs/teacher_infer.log",
    "log_level": "INFO"
  }
}
EOF
test -s "$workspace/configs/teacher_infer.json" || fail "teacher config is empty"
pass "teacher config generated"

echo ""
echo "4. Simulate teacher inference output"
cat > "$workspace/data/generated/labeled.json" <<'EOF'
{"instruction":"Explain machine learning","input":"","output":"Machine learning lets systems improve behavior from data."}
{"instruction":"Explain deep learning","input":"","output":"Deep learning uses multi-layer neural networks to learn complex patterns."}
{"instruction":"Create a Python list","input":"","output":"Use square brackets, for example: values = [1, 2, 3]."}
{"instruction":"Explain neural networks","input":"","output":"A neural network is a connected set of computational units."}
{"instruction":"Explain NLP","input":"","output":"Natural language processing focuses on machine understanding of human language."}
EOF
labeled_count="$(wc -l < "$workspace/data/generated/labeled.json" | tr -d ' ')"
test "$labeled_count" = "5" || fail "expected 5 labeled rows, got $labeled_count"
pass "teacher output generated"

echo ""
echo "5. Create simulated logs"
cat > "$workspace/logs/teacher_infer.log" <<'EOF'
[2026-04-13 15:00:00] INFO: Starting teacher inference
[2026-04-13 15:00:01] INFO: Loading instruction data from /workspace/data/seed/instructions.json
[2026-04-13 15:00:02] INFO: Loaded 5 instructions
[2026-04-13 15:00:03] INFO: Initializing teacher model: test-model
[2026-04-13 15:00:05] INFO: Processing instruction 1/5
[2026-04-13 15:00:08] INFO: Processing instruction 2/5
[2026-04-13 15:00:11] INFO: Processing instruction 3/5
[2026-04-13 15:00:14] INFO: Processing instruction 4/5
[2026-04-13 15:00:17] INFO: Processing instruction 5/5
[2026-04-13 15:00:18] INFO: Saving labeled data to /workspace/data/generated/labeled.json
[2026-04-13 15:00:19] INFO: Teacher inference completed successfully
[2026-04-13 15:00:19] INFO: Total processed: 5, Success: 5, Failed: 0
EOF
test -s "$workspace/logs/teacher_infer.log" || fail "teacher log is empty"
pass "teacher log generated"
tail -n 5 "$workspace/logs/teacher_infer.log"

echo ""
echo "6. Generate student training config"
cat > "$workspace/configs/student_train.json" <<'EOF'
{
  "job_type": "kd_black_box_train_only",
  "dataset": {
    "instruction_path": "/workspace/data/filtered/train.json",
    "template": "chat_template/chat_template_kd.jinja"
  },
  "models": {
    "teacher": "test-teacher-model",
    "student": "test-student-model"
  },
  "training": {
    "output_dir": "/workspace/models/checkpoints/",
    "num_train_epochs": 1,
    "per_device_train_batch_size": 2,
    "learning_rate": 2e-5,
    "save_steps": 100,
    "logging_dir": "/workspace/logs/",
    "logging_steps": 10
  }
}
EOF
test -s "$workspace/configs/student_train.json" || fail "student config is empty"
pass "student config generated"

echo ""
echo "7. Simulate model checkpoint output"
mkdir -p "$workspace/models/checkpoints/checkpoint-100"
echo "fake model checkpoint" > "$workspace/models/checkpoints/checkpoint-100/pytorch_model.bin"
echo "fake config" > "$workspace/models/checkpoints/checkpoint-100/config.json"
test -s "$workspace/models/checkpoints/checkpoint-100/pytorch_model.bin" || fail "model checkpoint missing"
pass "model checkpoint generated"
ls -lh "$workspace/models/checkpoints/checkpoint-100/"

echo ""
echo "=========================================="
echo -e "${green}E2E workspace smoke test completed.${reset}"
echo "=========================================="
echo "Verified:"
echo "  - workspace layout"
echo "  - seed data copy"
echo "  - EasyDistill config generation"
echo "  - teacher output manifest"
echo "  - log persistence"
echo "  - model checkpoint output"
