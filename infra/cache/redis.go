package cache

import (
	"context"
	"fmt"
	"log"

	"github.com/YuHangN/ragent-go/config"
	"github.com/redis/go-redis/v9"
)

// NewRedis 负责初始化 Redis 客户端，并在启动阶段验证连接。
func NewRedis(cfg *config.RedisConfig) *redis.Client {
	// 创建 Redis 客户端。
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}

	fmt.Println("成功连接到 Redis")
	return rdb
}
