package idgen

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	once sync.Once
	node *snowflake.Node
)

// Init 初始化全局 Snowflake 节点。
//
// Init 应在服务启动时调用一次；nodeID 取值范围由 snowflake 库约束为 0-1023。
func Init(nodeID int64) {
	once.Do(func() {
		var err error
		node, err = snowflake.NewNode(nodeID)
		if err != nil {
			panic(fmt.Sprintf("idgen: failed to create snowflake node %d: %v", nodeID, err))
		}
	})
}

// NewID 生成 int64 类型的 Snowflake ID，通常用于数据库主键。
func NewID() int64 {
	return node.Generate().Int64()
}

// NewStringID 生成字符串类型的 Snowflake ID，通常用于对外业务 ID。
func NewStringID() string {
	return node.Generate().String()
}
