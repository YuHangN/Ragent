-- Phase 2: knowledge base + document + chunk tables (backfill)
-- 对应 Java t_knowledge_base / t_knowledge_document / t_knowledge_chunk

-- 知识库主表
CREATE TABLE IF NOT EXISTS `t_knowledge_base` (
    `id`              BIGINT       NOT NULL COMMENT '主键ID（Snowflake）',
    `name`            VARCHAR(128) NOT NULL COMMENT '知识库名称',
    `embedding_model` VARCHAR(128) NOT NULL COMMENT '嵌入模型标识',
    `collection_name` VARCHAR(128) NOT NULL COMMENT 'Milvus Collection 名',
    `created_by`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人 username',
    `updated_by`      VARCHAR(64)           DEFAULT NULL COMMENT '修改人 username',
    `create_time`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted`         DATETIME              DEFAULT NULL COMMENT '软删除：NULL 未删除（gorm.DeletedAt）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_collection_name` (`collection_name`),
    KEY `idx_kb_name` (`name`),
    KEY `idx_kb_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='RAG 知识库表';

-- 文档表
CREATE TABLE IF NOT EXISTS `t_knowledge_document` (
    `id`               BIGINT        NOT NULL COMMENT '主键ID（Snowflake）',
    `kb_id`            BIGINT        NOT NULL COMMENT '所属知识库ID',
    `doc_name`         VARCHAR(256)  NOT NULL DEFAULT '' COMMENT '文档名称',
    `enabled`          TINYINT       NOT NULL DEFAULT 1 COMMENT '是否启用：0 禁用 1 启用',
    `chunk_count`      INT           NOT NULL DEFAULT 0 COMMENT '当前分块数量',
    `file_url`         VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '文件 URL',
    `file_type`        VARCHAR(32)   NOT NULL DEFAULT '' COMMENT '文件类型',
    `file_size`        BIGINT        NOT NULL DEFAULT 0 COMMENT '文件大小（字节）',
    `process_mode`     VARCHAR(32)   NOT NULL DEFAULT 'chunk' COMMENT '处理模式：chunk / pipeline',
    `status`           VARCHAR(32)   NOT NULL DEFAULT 'pending' COMMENT '状态：pending/running/success/failed',
    `source_type`      VARCHAR(32)            DEFAULT NULL COMMENT '来源类型：file / url',
    `source_location`  VARCHAR(1024)          DEFAULT NULL COMMENT '来源位置（URL）',
    `schedule_enabled` TINYINT                DEFAULT 0 COMMENT '定时拉取：0 否 1 是',
    `schedule_cron`    VARCHAR(128)           DEFAULT NULL COMMENT '定时拉取 cron 表达式',
    `chunk_strategy`   VARCHAR(32)            DEFAULT NULL COMMENT '分块策略：fixed / paragraph / pipeline',
    `chunk_config`     JSON                   DEFAULT NULL COMMENT '分块参数 JSON',
    `pipeline_id`      BIGINT                 DEFAULT NULL COMMENT '关联的 pipeline ID',
    `created_by`       VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '创建人 username',
    `updated_by`       VARCHAR(64)            DEFAULT NULL COMMENT '修改人 username',
    `create_time`      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted`          DATETIME               DEFAULT NULL COMMENT '软删除：NULL 未删除（gorm.DeletedAt）',
    PRIMARY KEY (`id`),
    KEY `idx_kb_id` (`kb_id`),
    KEY `idx_doc_status` (`status`),
    KEY `idx_doc_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='RAG 知识库文档表';

-- 分块表
CREATE TABLE IF NOT EXISTS `t_knowledge_chunk` (
    `id`           BIGINT      NOT NULL COMMENT '主键ID（Snowflake）',
    `kb_id`        BIGINT      NOT NULL COMMENT '所属知识库ID',
    `doc_id`       BIGINT      NOT NULL COMMENT '所属文档ID',
    `chunk_index`  INT         NOT NULL DEFAULT 0 COMMENT '分块序号（从 0 开始）',
    `content`      LONGTEXT    NOT NULL COMMENT '分块正文内容',
    `content_hash` VARCHAR(64)          DEFAULT NULL COMMENT '内容哈希（幂等/去重）',
    `char_count`   INT         NOT NULL DEFAULT 0 COMMENT '字符数',
    `token_count`  INT         NOT NULL DEFAULT 0 COMMENT 'Token 数（Phase 4.5-B 后填充）',
    `enabled`      TINYINT     NOT NULL DEFAULT 1 COMMENT '是否启用：0 禁用 1 启用',
    `created_by`   VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人 username',
    `updated_by`   VARCHAR(64)          DEFAULT NULL COMMENT '修改人 username',
    `create_time`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time`  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted`      DATETIME             DEFAULT NULL COMMENT '软删除：NULL 未删除（gorm.DeletedAt）',
    PRIMARY KEY (`id`),
    KEY `idx_doc_id` (`doc_id`),
    KEY `idx_kb_doc` (`kb_id`, `doc_id`),
    KEY `idx_chunk_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='RAG 知识库文档分块表';