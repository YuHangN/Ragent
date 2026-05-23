// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件封装 Conversation 与 Message 两张表的数据访问。消息从属于会话，因此
// 统一放在一个 Repo 接口中，便于上层按会话维度读写历史。
package conversation

import "gorm.io/gorm"

// ConversationRepo 是会话与消息的统一数据访问接口。
//
// 接口只暴露当前业务层需要的会话 CRUD、分页列表、消息追加和最近消息读取能力。
// 更复杂的查询应通过新增方法表达，避免上层直接拼接 SQL。
type ConversationRepo interface {
	CreateConversation(c *Conversation) error
	UpdateConversation(c *Conversation) error
	DeleteConversation(id int64) error
	FindConversationByID(id int64) (*Conversation, error)
	ListConversationsByUser(userID int64, limit, offset int) ([]Conversation, int64, error)

	AppendMessage(m *Message) error
	// ListMessages 返回会话最近 limit 条消息，并按 create_time 升序排列。
	ListMessages(conversationID int64, limit int) ([]Message, error)
}

type gormConversationRepo struct{ db *gorm.DB }

// NewConversationRepo 创建基于 GORM 的 ConversationRepo 实现。
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

// ListConversationsByUser 返回用户会话列表，按 update_time 倒序排列。
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

// ListMessages 读取最近 limit 条消息，并按 create_time 升序返回。
//
// 查询时先按时间倒序取最近 N 条，再在内存中反转，保证返回值可直接作为从早到晚
// 的多轮上下文。
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
