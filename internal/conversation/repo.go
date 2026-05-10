// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件提供 Conversation 与 Message 两张表的数据访问。两张表语义紧耦合
// （消息必属于会话），所以合并到一个 Repo 接口，避免上层为了拉历史多注入一个
// 依赖。GORM DeletedAt 字段使所有读路径默认排除软删除记录。
package conversation

import "gorm.io/gorm"

// ConversationRepo 是会话与消息的统一数据访问接口。
//
// 它故意只暴露 ConversationService 真正需要的方法：会话级 CRUD + 列表分页，
// 消息级 Append + 倒序拉最近 N 条。任何更复杂的查询（按时间区间、按 role
// 过滤等）都应该新增方法，避免上层直接拼 SQL。
type ConversationRepo interface {
	CreateConversation(c *Conversation) error
	UpdateConversation(c *Conversation) error
	DeleteConversation(id int64) error
	FindConversationByID(id int64) (*Conversation, error)
	ListConversationsByUser(userID int64, limit, offset int) ([]Conversation, int64, error)

	AppendMessage(m *Message) error
	// ListMessages 返回该会话最近 limit 条消息，按 create_time 升序返回，
	// 直接喂给 LLM 作为多轮上下文。
	ListMessages(conversationID int64, limit int) ([]Message, error)
}

type gormConversationRepo struct{ db *gorm.DB }

// NewConversationRepo 构造默认实现。
func NewConversationRepo(db *gorm.DB) ConversationRepo {
	return &gormConversationRepo{db: db}
}

func (r *gormConversationRepo) CreateConversation(c *Conversation) error {
	return r.db.Create(c).Error
}

func (r *gormConversationRepo) UpdateConversation(c *Conversation) error {
	return r.db.Save(c).Error
}

func (r *gormConversationRepo) DeleteConversation(id int64) error {
	return r.db.Delete(&Conversation{}, id).Error
}

func (r *gormConversationRepo) FindConversationByID(id int64) (*Conversation, error) {
	var c Conversation
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListConversationsByUser 返回用户名下会话列表，按 update_time 倒序，
// 与 ChatGPT 风格一致：最近活跃的会话排在最上面。
func (r *gormConversationRepo) ListConversationsByUser(userID int64, limit, offset int) ([]Conversation, int64, error) {
	var list []Conversation
	var total int64
	if err := r.db.Model(&Conversation{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("user_id = ?", userID).
		Order("update_time DESC").
		Limit(limit).Offset(offset).
		Find(&list).Error
	return list, total, err
}

func (r *gormConversationRepo) AppendMessage(m *Message) error {
	return r.db.Create(m).Error
}

// ListMessages 取最近 limit 条消息，按 create_time 升序返回。
//
// SQL 语义上是先按时间倒序拉 N 条（避开全表扫描），再原地反转交给上层，
// 保证 chat 历史是"早 → 晚"顺序，方便直接拼 LLM 多轮上下文。
func (r *gormConversationRepo) ListMessages(conversationID int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	var msgs []Message
	err := r.db.Where("conversation_id = ?", conversationID).
		Order("create_time DESC, id DESC").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}
