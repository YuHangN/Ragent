-- Phase 3.5-B: knowledge schedule + chunklog tables (new in this phase)

-- 文档定时调度配置表
CREATE TABLE IF NOT EXISTS `t_knowledge_document_schedule` (
    `id`                BIGINT       NOT NULL COMMENT '主键ID（Snowflake）',
    `doc_id`            BIGINT       NOT NULL COMMENT '关联的文档ID',
    `kb_id`             BIGINT       NOT NULL COMMENT '所属知识库ID',
    `cron_expr`         VARCHAR(255)          DEFAULT NULL COMMENT 'cron 表达式',
    `enabled`           TINYINT      NOT NULL DEFAULT 1 COMMENT '是否启用：0 禁用 1 启用',
    `next_run_time`     DATETIME              DEFAULT NULL COMMENT '下次执行时间',
    `last_run_time`     DATETIME              DEFAULT NULL COMMENT '上次执行时间',
    `last_success_time` DATETIME              DEFAULT NULL COMMENT '上次成功时间',
    `last_status`       VARCHAR(32)           DEFAULT NULL COMMENT '上次执行状态',
    `last_error`        VARCHAR(512)          DEFAULT NULL COMMENT '上次执行错误信息',
    `last_etag`         VARCHAR(255)          DEFAULT NULL COMMENT '上次 ETag（幂等）',
    `last_modified`     VARCHAR(255)          DEFAULT NULL COMMENT '上次 Last-Modified（幂等）',
    `last_content_hash` VARCHAR(128)          DEFAULT NULL COMMENT '上次内容哈希（幂等）',
    `lock_owner`        VARCHAR(128)          DEFAULT NULL COMMENT '分布式锁持有者',
    `lock_until`        DATETIME              DEFAULT NULL COMMENT '锁过期时间',
    `create_time`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_doc_id` (`doc_id`),
    KEY `idx_kb_id` (`kb_id`),
    KEY `idx_lock_until` (`lock_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档定时调度配置表（无 deleted 列：实体用 enabled 标志位，不走软删除）';

-- 文档定时执行记录表
CREATE TABLE IF NOT EXISTS `t_knowledge_document_schedule_exec` (
    `id`            BIGINT        NOT NULL COMMENT '主键ID（Snowflake）',
    `schedule_id`   BIGINT        NOT NULL COMMENT '关联调度配置ID',
    `doc_id`        BIGINT        NOT NULL COMMENT '关联文档ID',
    `kb_id`         BIGINT        NOT NULL COMMENT '所属知识库ID',
    `status`        VARCHAR(32)            DEFAULT NULL COMMENT '执行状态：success/failed/skipped',
    `message`       VARCHAR(1024)          DEFAULT NULL COMMENT '执行说明/错误信息',
    `start_time`    DATETIME               DEFAULT NULL COMMENT '开始时间',
    `end_time`      DATETIME               DEFAULT NULL COMMENT '结束时间',
    `file_name`     VARCHAR(255)           DEFAULT NULL COMMENT '本次拉取文件名',
    `file_size`     BIGINT                 DEFAULT NULL COMMENT '本次文件大小（字节）',
    `content_hash`  VARCHAR(128)           DEFAULT NULL COMMENT '本次内容哈希',
    `etag`          VARCHAR(255)           DEFAULT NULL COMMENT '本次 ETag',
    `last_modified` VARCHAR(255)           DEFAULT NULL COMMENT '本次 Last-Modified',
    `create_time`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_schedule_id` (`schedule_id`),
    KEY `idx_doc_id` (`doc_id`),
    KEY `idx_exec_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档定时执行记录表';

-- 文档切片日志表
CREATE TABLE IF NOT EXISTS `t_knowledge_document_chunk_log` (
    `id`                 BIGINT        NOT NULL COMMENT '主键ID（Snowflake）',
    `doc_id`             BIGINT        NOT NULL COMMENT '关联文档ID',
    `status`             VARCHAR(32)            DEFAULT NULL COMMENT '状态：running/success/failed',
    `process_mode`       VARCHAR(32)            DEFAULT NULL COMMENT '处理模式：chunk / pipeline',
    `chunk_strategy`     VARCHAR(64)            DEFAULT NULL COMMENT '分块策略快照',
    `pipeline_id`        BIGINT                 DEFAULT NULL COMMENT '关联 pipeline ID',
    `extract_duration`   BIGINT                 DEFAULT NULL COMMENT '抽取耗时（毫秒）',
    `chunk_duration`     BIGINT                 DEFAULT NULL COMMENT '切片耗时（毫秒）',
    `embedding_duration` BIGINT                 DEFAULT NULL COMMENT '嵌入耗时（毫秒）',
    `total_duration`     BIGINT                 DEFAULT NULL COMMENT '总耗时（毫秒）',
    `chunk_count`        INT                    DEFAULT NULL COMMENT '切片数量',
    `error_message`      VARCHAR(1024)          DEFAULT NULL COMMENT '错误信息',
    `start_time`         DATETIME               DEFAULT NULL COMMENT '开始时间',
    `end_time`           DATETIME               DEFAULT NULL COMMENT '结束时间',
    `create_time`        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_doc_id` (`doc_id`),
    KEY `idx_chunklog_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文档切片日志表';