package idgen

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
	"sync"
)

var (
	once sync.Once
	node *snowflake.Node
)

// Init 在 main.go 启动时调用一次，nodeID 取值范围 0–1023。
func Init(nodeID int64) {
	once.Do(func() {
		var err error
		node, err = snowflake.NewNode(nodeID)
		if err != nil {
			panic(fmt.Sprintf("idgen: failed to create snowflake node %d: %v", nodeID, err))
		}
	})
}

// NewID 返回 int64 类型的 Snowflake ID，用于数据库主键。
func NewID() int64 {
	return node.Generate().Int64()
}

// NewStringID 返回字符串类型的 Snowflake ID，用于 chunkId 等业务 ID。
func NewStringID() string {
	return node.Generate().String()
}
