// Package conversation 提供 RAG Chat 的会话管理与问答链路。
//
// 本文件提供应用层的 ConversationService，封装会话创建、消息追加、历史加载和
// 标题修改。RAG 与 LLM 调用由 ChatService 负责，这里只处理会话与消息的持久化
// 规则。
package conversation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
)

// titleAutoFillMaxRunes 是自动生成标题的最大 rune 数。
//
// 超出长度时会追加省略号，避免自动标题过长影响列表展示。
const titleAutoFillMaxRunes = 30

// ConversationService 是会话与消息的应用层入口。
//
// 它不持有全局状态，所有数据读写都委托给 ConversationRepo。
type ConversationService struct {
	repo ConversationRepo
}

// NewConversationService 创建 ConversationService。
func NewConversationService(repo ConversationRepo) *ConversationService {
	return &ConversationService{repo: repo}
}

// CreateSession 创建一个会话。
//
// kbIDs 会序列化为 JSON 字符串保存；title 为空时可在首条用户消息追加时回填。
// 返回值为写入后的 Conversation，包含已分配的 ID。
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
// 当 role=user 且会话标题为空时，会使用消息内容生成标题。标题更新失败不影响
// 消息写入，因为消息持久化是主路径，标题只是衍生展示数据。
//
// chunksJSON 通常只在 assistant 消息中传入，角色与 chunksJSON 的搭配由上层
// ChatService 控制。
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

// LoadHistory 按时间正序返回最近 limit 条消息，并转换为 aiclient.ChatMessage。
//
// 返回值可直接作为 RAG 或 LLM 的多轮上下文；limit <= 0 时由 Repo 使用默认值。
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

// RenameTitle 将会话标题改为指定值。
//
// 显式重命名不做自动截断，输入长度由调用方或前端表单控制。
func (s *ConversationService) RenameTitle(conversationID int64, title string) error {
	conv, err := s.repo.FindConversationByID(conversationID)
	if err != nil {
		return err
	}
	conv.Title = title
	return s.repo.UpdateConversation(conv)
}

// truncateTitle 将内容裁剪到最多 maxRunes 个 rune，超出时追加省略号。
//
// 使用 []rune 处理，避免截断中文等多字节字符。
func truncateTitle(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
