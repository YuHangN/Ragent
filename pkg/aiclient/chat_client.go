package aiclient

import (
	"context"
)

// ChatClient 定义聊天补全 provider 适配器需要实现的接口。
type ChatClient interface {
	Provider() Provider
	Chat(ctx context.Context, req ChatRequest, target *ModelTarget) (string, error)
	StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback, target *ModelTarget) error
}
