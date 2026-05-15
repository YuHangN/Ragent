-- Phase 6.7: Collection → Partition refactor
--
-- 改动 1：t_intent_node 把 collection_name 列重命名为 partition_name
--         语义从「Milvus collection 名」改为「该意图节点对应的 partition 名」
-- 改动 2：t_knowledge_document 加 target_partition 列
--         上传文档时指定写入哪个 partition；空 / _default 表示默认分区

ALTER TABLE t_intent_node
    CHANGE COLUMN collection_name partition_name VARCHAR(255) DEFAULT NULL
    COMMENT 'Kind=KB 时填，对应 KB collection 下的 Milvus partition 名';

ALTER TABLE t_knowledge_document
    ADD COLUMN target_partition VARCHAR(255) NOT NULL DEFAULT '_default'
    COMMENT '文档要写入的 Milvus partition 名；空 / _default 走 collection 默认分区';

-- 回滚（如需手工执行）：
-- ALTER TABLE t_knowledge_document DROP COLUMN target_partition;
-- ALTER TABLE t_intent_node
--     CHANGE COLUMN partition_name collection_name VARCHAR(255) DEFAULT NULL;
