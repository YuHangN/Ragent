// Package rag 包含检索增强生成（RAG）相关的核心业务逻辑。
//
// 本文件负责构建 RAG 路由层使用的“意图树”。
// 数据库中的意图节点是扁平存储的，每一行只记录自己的 ID 和可选的 parent_id；
// 但 API 调用方和前端页面通常需要树形结构。因此，本文件会把扁平的 Node
// 记录转换成带有 Children 子节点的 NodeTreeVO。
//
// 通俗来说：这个文件就是把“数据库里一行一行的意图记录”，整理成“人和程序都更容易
// 理解的意图菜单树”。如果某个节点没有父节点，或者它的父节点不在本次输入数据里，
// 该节点会被当作根节点处理，避免因为数据不完整而静默丢失节点。
package intent

import "fmt"

// NodeTreeVO 是面向 API 响应的意图节点树形结构。
//
// 服务层内部使用扁平的 Node 模型存储意图节点；该结构会保留相同的业务字段，
// 将数字 ID 格式化为字符串，便于 JSON 客户端稳定处理，并通过 Children 递归挂载
// 子节点。
type NodeTreeVO struct {
	ID             string             `json:"id"`
	KbID           string             `json:"kbId"`
	ParentID       string             `json:"parentId,omitempty"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Examples       string             `json:"examples,omitempty"`
	Level          int                `json:"level"`
	Kind           Kind         `json:"kind"`
	PartitionName  string             `json:"partitionName,omitempty"`
	MCPToolID      string             `json:"mcpToolId,omitempty"`
	TopK           *int               `json:"topK,omitempty"`
	Enabled        bool               `json:"enabled"`
	SortOrder      int                `json:"sortOrder"`
	Children       []NodeTreeVO `json:"children"`
}

// BuildTree 将扁平的意图节点列表转换成嵌套的意图树。
//
// 父子关系由 Node.ParentID 决定。没有父节点的节点会成为根节点；如果某个节点
// 指向的父节点不在本次输入列表中，该节点也会被提升为根节点。这样即使查询结果不完整，
// 也能保证节点仍然出现在返回结果里，不会被静默丢弃。
func BuildTree(nodes []Node) []NodeTreeVO {
	if len(nodes) == 0 {
		return nil
	}
	voByID := make(map[int64]*NodeTreeVO, len(nodes))
	for i := range nodes {
		vo := toTreeVO(&nodes[i])
		voByID[nodes[i].ID] = &vo
	}
	childrenOf := make(map[int64][]int64)
	var rootIDs []int64
	for _, n := range nodes {
		if n.ParentID == nil {
			rootIDs = append(rootIDs, n.ID)
			continue
		}
		if _, ok := voByID[*n.ParentID]; !ok {
			rootIDs = append(rootIDs, n.ID)
			continue
		}
		childrenOf[*n.ParentID] = append(childrenOf[*n.ParentID], n.ID)
	}
	var build func(id int64) NodeTreeVO
	build = func(id int64) NodeTreeVO {
		vo := *voByID[id]
		for _, cid := range childrenOf[id] {
			vo.Children = append(vo.Children, build(cid))
		}
		return vo
	}
	roots := make([]NodeTreeVO, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, build(id))
	}
	return roots
}

// toTreeVO 将数据库存储模型转换成 JSON 响应模型。
//
// 这里会把数字 ID 转成字符串，避免部分客户端无法安全处理大整数 ID；同时会把数据库中的
// enabled 标记转换成布尔值，并把 Children 初始化为空切片，保证 JSON 输出结构稳定。
func toTreeVO(n *Node) NodeTreeVO {
	vo := NodeTreeVO{
		ID:             fmt.Sprintf("%d", n.ID),
		KbID:           fmt.Sprintf("%d", n.KbID),
		Name:           n.Name,
		Description:    n.Description,
		Examples:       n.Examples,
		Level:          n.Level,
		Kind:           n.Kind,
		PartitionName:  n.PartitionName,
		MCPToolID:      n.MCPToolID,
		TopK:           n.TopK,
		Enabled:        n.Enabled == 1,
		SortOrder:      n.SortOrder,
		Children:       []NodeTreeVO{},
	}
	if n.ParentID != nil {
		vo.ParentID = fmt.Sprintf("%d", *n.ParentID)
	}
	return vo
}
