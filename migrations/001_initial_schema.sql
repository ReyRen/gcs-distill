-- gcs-distill 数据库初始化脚本
-- 版本: 001
-- 描述: 创建项目、数据集、流水线运行、阶段运行、容器运行、评估报告表

-- 创建数据库（如果不存在）
-- CREATE DATABASE gcs_distill;

-- 扩展: 启用 UUID 支持
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 表 1: 蒸馏项目表
CREATE TABLE IF NOT EXISTS distill_projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    business_scenario VARCHAR(255),
    teacher_model_config JSONB NOT NULL,  -- 教师模型配置
    student_model_config JSONB NOT NULL,  -- 学生模型配置
    evaluation_config JSONB,               -- 评估配置
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_projects_name ON distill_projects(name);
CREATE INDEX idx_projects_created_at ON distill_projects(created_at DESC);

COMMENT ON TABLE distill_projects IS '蒸馏项目表';
COMMENT ON COLUMN distill_projects.teacher_model_config IS '教师模型配置 (JSON)';
COMMENT ON COLUMN distill_projects.student_model_config IS '学生模型配置 (JSON)';

-- 表 2: 数据集表
CREATE TABLE IF NOT EXISTS datasets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES distill_projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    source_type VARCHAR(50) NOT NULL,  -- upload, import, generated
    file_path TEXT NOT NULL,            -- 文件存储路径
    record_count INT DEFAULT 0,         -- 记录数
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_datasets_project_id ON datasets(project_id);
CREATE INDEX idx_datasets_created_at ON datasets(created_at DESC);

COMMENT ON TABLE datasets IS '数据集表';
COMMENT ON COLUMN datasets.source_type IS '数据来源: upload(上传), import(导入), generated(生成)';

-- 表 3: 流水线运行表
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES distill_projects(id) ON DELETE CASCADE,
    dataset_id UUID NOT NULL REFERENCES datasets(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, scheduled, preparing, running, succeeded, failed, canceled
    current_stage INT DEFAULT 0,                     -- 当前阶段 (0-6)
    trigger_mode VARCHAR(50) DEFAULT 'manual',       -- manual, scheduled
    training_config JSONB NOT NULL,                  -- 训练配置
    resource_request JSONB,                          -- 资源请求
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_pipeline_runs_project_id ON pipeline_runs(project_id);
CREATE INDEX idx_pipeline_runs_status ON pipeline_runs(status);
CREATE INDEX idx_pipeline_runs_created_at ON pipeline_runs(created_at DESC);

COMMENT ON TABLE pipeline_runs IS '流水线运行实例表';
COMMENT ON COLUMN pipeline_runs.current_stage IS '当前阶段: 0=未开始, 1-6=六个阶段';

-- 表 4: 阶段运行表
CREATE TABLE IF NOT EXISTS stage_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    stage_type VARCHAR(50) NOT NULL,  -- teacher_config, dataset_build, teacher_infer, data_govern, student_train, evaluate
    stage_order INT NOT NULL,          -- 阶段序号 (1-6)
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    container_id VARCHAR(255),         -- 容器 ID
    node_name VARCHAR(255),            -- 节点名称
    config_path TEXT,                  -- EasyDistill 配置文件路径
    input_manifest JSONB,              -- 输入清单
    output_manifest JSONB,             -- 输出清单
    metrics JSONB,                     -- 指标数据
    log_path TEXT,                     -- 日志路径
    retry_count INT DEFAULT 0,         -- 重试次数
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_stage_runs_pipeline_run_id ON stage_runs(pipeline_run_id);
CREATE INDEX idx_stage_runs_stage_type ON stage_runs(stage_type);
CREATE INDEX idx_stage_runs_status ON stage_runs(status);
CREATE INDEX idx_stage_runs_stage_order ON stage_runs(pipeline_run_id, stage_order);

COMMENT ON TABLE stage_runs IS '阶段运行实例表';
COMMENT ON COLUMN stage_runs.stage_type IS '阶段类型: teacher_config, dataset_build, teacher_infer, data_govern, student_train, evaluate';

-- 表 5: 容器运行表
CREATE TABLE IF NOT EXISTS container_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stage_run_id UUID NOT NULL REFERENCES stage_runs(id) ON DELETE CASCADE,
    container_name VARCHAR(255) NOT NULL,
    image VARCHAR(255) NOT NULL,
    node_name VARCHAR(255) NOT NULL,
    node_addr VARCHAR(255) NOT NULL,
    command TEXT,
    args JSONB,                        -- 参数数组
    envs JSONB,                        -- 环境变量
    mounts JSONB,                      -- 挂载配置
    xpu_allocation VARCHAR(255),       -- GPU/NPU 分配
    exit_code INT,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_container_runs_stage_run_id ON container_runs(stage_run_id);
CREATE INDEX idx_container_runs_node_name ON container_runs(node_name);
CREATE INDEX idx_container_runs_created_at ON container_runs(created_at DESC);

COMMENT ON TABLE container_runs IS '容器运行实例表';
COMMENT ON COLUMN container_runs.xpu_allocation IS 'GPU/NPU 分配信息';

-- 表 6: 评估报告表
CREATE TABLE IF NOT EXISTS evaluation_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    stage_run_id UUID NOT NULL REFERENCES stage_runs(id),
    metrics JSONB NOT NULL,            -- 评估指标
    details JSONB,                     -- 详细信息
    summary TEXT,                      -- 总结
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_evaluation_reports_pipeline_run_id ON evaluation_reports(pipeline_run_id);
CREATE INDEX idx_evaluation_reports_stage_run_id ON evaluation_reports(stage_run_id);
CREATE INDEX idx_evaluation_reports_created_at ON evaluation_reports(created_at DESC);

COMMENT ON TABLE evaluation_reports IS '评估报告表';
COMMENT ON COLUMN evaluation_reports.metrics IS '评估指标 (BLEU, ROUGE, Accuracy 等)';

-- 表 7: 产物表 (用于索引训练产物和模型)
CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    stage_run_id UUID REFERENCES stage_runs(id),
    artifact_type VARCHAR(50) NOT NULL,  -- checkpoint, model, log, dataset, report
    name VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT,                     -- 文件大小 (bytes)
    metadata JSONB,                       -- 元数据
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_artifacts_pipeline_run_id ON artifacts(pipeline_run_id);
CREATE INDEX idx_artifacts_stage_run_id ON artifacts(stage_run_id);
CREATE INDEX idx_artifacts_artifact_type ON artifacts(artifact_type);
CREATE INDEX idx_artifacts_created_at ON artifacts(created_at DESC);

COMMENT ON TABLE artifacts IS '产物表 (模型、日志、数据集等)';
COMMENT ON COLUMN artifacts.artifact_type IS '产物类型: checkpoint, model, log, dataset, report';

-- 触发器: 自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_distill_projects_updated_at BEFORE UPDATE ON distill_projects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_pipeline_runs_updated_at BEFORE UPDATE ON pipeline_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_stage_runs_updated_at BEFORE UPDATE ON stage_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 初始化完成
SELECT 'Database schema initialized successfully' AS status;
