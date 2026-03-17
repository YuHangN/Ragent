package vector

import (
	"context"
	"fmt"
	"log"

	"github.com/YuHangN/ragent-go/config"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// NewMilvus 负责初始化 Milvus 客户端。
func NewMilvus(cfg *config.MilvusConfig) client.Client {
	// 创建 Milvus 客户端。
	c, err := client.NewClient(context.Background(), client.Config{
		Address: cfg.URI,
	})
	if err != nil {
		log.Fatalf("连接 Milvus 失败: %v", err)
	}

	// 测试连接是否成功。
	if _, err := c.GetVersion(context.Background()); err != nil {
		log.Fatalf("测试 Milvus 连接失败: %v", err)
	}

	fmt.Println("成功连接到 Milvus")
	return c
}
