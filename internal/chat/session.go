package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/coder/websocket"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/muaathrifath/sol-core/internal/user"
)

const systemInstructions = "You are Sol, a smart home AI assistant. " +
	"For device control requests, with NO text before tool calls: (1) call discover_devices; " +
	"(2) call check_device_online on the chosen appliance; " +
	"(3) call get_device_state to see the current state; " +
	"(4) call control_device only when the requested action would change state. " +
	"After control_device, report exactly what its ok and message fields say - never claim success when ok is false. " +
	"For \"is it on?\" questions, use get_device_state. " +
	"For greetings or non-device questions, answer directly. " +
	"Be concise."

// SessionConfig holds Azure OpenAI connection parameters.
type SessionConfig struct {
	Endpoint   string
	APIKey     string
	Deployment string
}

// Session runs the agentic chat loop for one frontend WebSocket connection.
type Session struct {
	frontend *websocket.Conn
	u        *user.User
	homeID   string
	tools    *Tools
	cfg      SessionConfig
	messages []openai.ChatCompletionMessageParamUnion
}

func NewSession(frontend *websocket.Conn, u *user.User, homeID string, tools *Tools, cfg SessionConfig) *Session {
	return &Session{
		frontend: frontend,
		u:        u,
		homeID:   homeID,
		tools:    tools,
		cfg:      cfg,
	}
}

// Run reads frontend WebSocket messages and drives the completions loop.
// Blocks until the connection closes or ctx is cancelled.
func (s *Session) Run(ctx context.Context) error {
	client := openai.NewClient(
		option.WithBaseURL(s.cfg.Endpoint),
		option.WithAPIKey(s.cfg.APIKey),
	)

	for {
		_, raw, err := s.frontend.Read(ctx)
		if err != nil {
			return err
		}

		var event map[string]any
		if json.Unmarshal(raw, &event) != nil {
			continue
		}

		switch event["type"] {
		case "conversation.item.create":
			s.addFromItem(event)
		case "response.create":
			if err := s.runCompletions(ctx, client); err != nil {
				slog.Warn("chat/session: completions error", "error", err)
			}
		}
	}
}

// addFromItem parses a conversation.item.create event (Realtime-protocol format)
// and appends the message to the session history.
func (s *Session) addFromItem(event map[string]any) {
	item, _ := event["item"].(map[string]any)
	if item == nil {
		return
	}
	role, _ := item["role"].(string)

	var b strings.Builder
	if arr, ok := item["content"].([]any); ok {
		for _, c := range arr {
			if cm, ok := c.(map[string]any); ok {
				if t, ok := cm["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
	}
	text := b.String()

	switch role {
	case "user":
		s.messages = append(s.messages, openai.UserMessage(text))
	case "assistant":
		s.messages = append(s.messages, openai.AssistantMessage(text))
	}
}

// runCompletions drives the agentic loop: stream a completion, execute any tool
// calls, and repeat until the model produces a final text-only response.
func (s *Session) runCompletions(ctx context.Context, client openai.Client) error {
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(s.messages)+1)
	history = append(history, openai.SystemMessage(systemInstructions))
	history = append(history, s.messages...)

	for {
		if err := s.emit(ctx, map[string]any{"type": "response.created"}); err != nil {
			return err
		}

		stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(s.cfg.Deployment),
			Messages: history,
			Tools:    toolSchemaParams(),
		})

		var acc openai.ChatCompletionAccumulator

		for stream.Next() {
			acc.AddChunk(stream.Current())
		}
		if err := stream.Err(); err != nil {
			return fmt.Errorf("chat/session: stream: %w", err)
		}

		if len(acc.Choices) == 0 {
			return nil
		}

		msg := acc.Choices[0].Message
		history = append(history, msg.ToParam())

		toolCalls := msg.ToolCalls
		if len(toolCalls) == 0 {
			// Final text response — emit now that we know there are no tool calls.
			text := msg.Content
			if text != "" {
				if err := s.emit(ctx, map[string]any{"type": "response.text.delta", "delta": text}); err != nil {
					return err
				}
				if err := s.emit(ctx, map[string]any{"type": "response.text.done", "text": text}); err != nil {
					return err
				}
			}
			if err := s.emit(ctx, map[string]any{"type": "response.done"}); err != nil {
				return err
			}
			s.messages = append(s.messages, msg.ToParam())
			return nil
		}

		if err := s.emit(ctx, map[string]any{"type": "response.done"}); err != nil {
			return err
		}

		for _, tc := range toolCalls {
			slog.Info("chat: tool call", "tool", tc.Function.Name, "home", s.homeID, "user", s.u.ID)
			result := s.tools.Dispatch(ctx, tc.Function.Name, tc.Function.Arguments, s.u, ToolContext{
				HomeID:    s.homeID,
				ActorType: "user",
				ActorID:   s.u.ID,
			})
			history = append(history, openai.ToolMessage(result, tc.ID))
		}
	}
}

func (s *Session) emit(ctx context.Context, v map[string]any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.frontend.Write(ctx, websocket.MessageText, b)
}
