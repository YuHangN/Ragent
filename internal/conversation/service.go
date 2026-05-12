// Package conversation 实现 RAG Chat 的会话管理与对话主链路。
//
// 本文件提供应用层的 ConversationService，封装会话与消息的常用操作：
// 创建会话、追加消息、加载历史、改标题。它不直接处理 RAG 或 LLM 调用——那是
// ChatService 的责任；这里只关心"会话 + 消息"这一层的持久化与小型业务规则
// （比如首条 user 消息自动写 title）。
package conversation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// titleAutoFillMaxRunes 控制自动生成 title 的最大字符数。
//
// 取 30 个 rune 是经验值：太短截不到关键信息，太长侧边栏显示溢出。
// 超出长度的标题后面会加 "…" 提示截断。
const titleAutoFillMaxRunes = 30

// ConversationService 是会话与消息的应用层入口。
//
// 它不持有任何全局状态，所有数据都从 ConversationRepo 读取/写入；测试时只需
// mock 一个 repo 实现即可覆盖业务规则（自动 title、历史加载等）。
type ConversationService struct {
	repo ConversationRepo
}

// NewConversationService 构造 ConversationService。
func NewConversationService(repo ConversationRepo) *ConversationService {
	return &ConversationService{repo: repo}
}

// CreateSession 新建一个会话。
//
// kbIDs 序列化为 JSON 字符串持久化，避免新建一张关联表；title 为空时会等首条
// user 消息追加时由 AppendMessage 自动回填。返回值是落库后的 Conversation
// （含 snowflake ID）。
func (s *ConversationService) CreateSession(userID int64, kbIDs []int64, title string) (*Conversation, error) {
	kbJSON, _ := json.Marshal(kbIDs)
	c := &Conversation{
		UserID: userID,
		Title:  title,
		KbIDs:  string(kbJSON),
	}
	if err := s.repo.CreateConversation(c); err != nil {
		return nil, err
	}
	return c, nil
}

// AppendMessage 在会话中追加一条消息。
//
// 业务规则：若 role=user 且会话当前 title 为空，自动用问题截断 30 字写回会话
// title（ChatGPT 风格的"首条问题作为侧边栏摘要"）。title 更新失败不影响消息
// 写入——消息持久化是主路径，title 是衍生显示数据。
//
// chunksJSON 仅在 role=assistant 时建议传值（RAG 召回元信息）；其它角色应传空串。
// 该方法不强制校验角色与 chunksJSON 的搭配，由上层 ChatService 控制。
func (s *ConversationService) AppendMessage(conversationID int64, role aiclient.Role, content, chunksJSON string) (*Message, error) {
	conv, err := s.repo.FindConversationByID(conversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation %d not found: %w", conversationID, err)
	}

	m := &Message{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		ChunksJSON:     chunksJSON,
	}
	if err := s.repo.AppendMessage(m); err != nil {
		return nil, err
	}

	if role == aiclient.RoleUser && conv.Title == "" {
		conv.Title = truncateTitle(content, titleAutoFillMaxRunes)
		_ = s.repo.UpdateConversation(conv)
	}
	return m, nil
}

// LoadHistory 按时间正序返回最近 limit 条消息，转换为 aiclient.ChatMessage。
//
// 直接喂给 RAGCoreService.Retrieve 或 LLMService.Chat 作为多轮上下文。
// limit ≤ 0 时由 Repo 兜底（当前默认 20）。
func (s *ConversationService) LoadHistory(conversationID int64, limit int) ([]aiclient.ChatMessage, error) {
	msgs, err := s.repo.ListMessages(conversationID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]aiclient.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, aiclient.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

// RenameTitle 把会话标题改成指定值，供前端"重命名会话"按钮使用。
//
// 不做长度截断——既然用户显式重命名了，就以用户输入为准；前端如果想限长，自己
// 在表单层校验。
func (s *ConversationService) RenameTitle(conversationID int64, title string) error {
	conv, err := s.repo.FindConversationByID(conversationID)
	if err != nil {
		return err
	}
	conv.Title = title
	return s.repo.UpdateConversation(conv)
}

// truncateTitle 把内容裁成最多 maxRunes 个字符，超出则在末尾加 "…"。
//
// 用 []rune 而不是 len(string) 是为了避免中文等多字节字符被切断。
func truncateTitle(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
