-- Phase 1: user table (backfill, originally created via GORM AutoMigrate)
-- 对应 Java t_user，但去掉 AUTO_INCREMENT（Go 端用 Snowflake）

CREATE TABLE IF NOT EXISTS `t_user` (
    `id`          BIGINT       NOT NULL COMMENT '主键ID（Snowflake）',
    `username`    VARCHAR(64)  NOT NULL COMMENT '用户名，唯一',
    `password`    VARCHAR(128) NOT NULL COMMENT '密码（BCrypt 哈希）',
    `role`        VARCHAR(32)  NOT NULL DEFAULT 'user' COMMENT '角色：admin/user',
    `avatar`      VARCHAR(128)          DEFAULT NULL COMMENT '用户头像 URL',
    `create_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted`     DATETIME              DEFAULT NULL COMMENT '软删除：NULL 未删除，写入删除时间表示已删除（gorm.DeletedAt）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_username` (`username`),
    KEY `idx_user_deleted` (`deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统用户表';