package openai

import (
	"context"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	ai "github.com/openai/openai-go"
)

func TestNewClient(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
		Model:   "gpt-3.5-turbo",
	}
	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient 返回了 nil")
	}
}

func TestChatCompletion(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
		Model:   "gpt-3.5-turbo",
	}
	client := NewClient(cfg)
	messages := []ai.ChatCompletionMessageParamUnion{
		{
			OfUser: &ai.ChatCompletionUserMessageParam{
				Content: ai.ChatCompletionUserMessageParamContentUnion{
					OfString: ai.String("你好"),
				},
			},
		},
	}
	result, err := client.ChatCompletion(context.Background(), messages)
	if err != nil {
		t.Logf("ChatCompletion 返回错误（预期外部API会失败）: %v", err)
	}
	if result == nil {
		t.Fatal("ChatCompletion 返回了 nil")
	}
}

func TestChatCompletionStream(t *testing.T) {
	cfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com",
		Model:   "gpt-3.5-turbo",
	}
	client := NewClient(cfg)
	messages := []ai.ChatCompletionMessageParamUnion{
		{
			OfUser: &ai.ChatCompletionUserMessageParam{
				Content: ai.ChatCompletionUserMessageParamContentUnion{
					OfString: ai.String("你好"),
				},
			},
		},
	}
	_, err := client.ChatCompletionStream(context.Background(), messages, nil)
	if err != nil {
		t.Logf("ChatCompletionStream 返回错误（预期外部API会失败）: %v", err)
	}
}
