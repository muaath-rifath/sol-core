package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/muaathrifath/sol-core/internal/user"
)

// SessionConfig holds the Azure OpenAI Realtime API connection parameters.
type SessionConfig struct {
	AzureEndpoint string
	AzureKey      string
	Deployment    string
	APIVersion    string
}

// Session bridges a frontend WebSocket connection to the Azure OpenAI Realtime API.
type Session struct {
	frontend *websocket.Conn
	u        *user.User
	homeID   string
	tools    *Tools
	cfg      SessionConfig
}

func NewSession(frontend *websocket.Conn, u *user.User, homeID string, tools *Tools, cfg SessionConfig) *Session {
	return &Session{frontend: frontend, u: u, homeID: homeID, tools: tools, cfg: cfg}
}

// Run connects to Azure Realtime, configures the session, then fans-in both connections.
// Blocks until either side closes or ctx is cancelled.
func (s *Session) Run(ctx context.Context) error {
	// Build Azure Realtime WS URL.
	base := strings.TrimPrefix(strings.TrimPrefix(s.cfg.AzureEndpoint, "https://"), "http://")
	azureURL := fmt.Sprintf("wss://%s/openai/realtime?api-version=%s&deployment=%s", base, s.cfg.APIVersion, s.cfg.Deployment)

	upstream, _, err := websocket.Dial(ctx, azureURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"api-key": []string{s.cfg.AzureKey}},
	})
	if err != nil {
		return fmt.Errorf("chat/session: dial azure: %w", err)
	}
	defer upstream.CloseNow()

	// Configure the Realtime session: text-only, register tools.
	sessionUpdate := map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"modalities": []string{"text"},
			"tools":      toolSchemas(),
			"tool_choice": "auto",
			"instructions": "You are Sol, a smart home AI assistant. " +
				"For device control requests: call discover_devices immediately with NO preceding text, " +
				"then call control_device using the returned appliance_id. " +
				"Only generate text after all tool calls are complete. " +
				"For greetings or non-device questions, respond directly without calling any tools. " +
				"Be concise.",
		},
	}
	if err := writeJSON(ctx, upstream, sessionUpdate); err != nil {
		return fmt.Errorf("chat/session: send session.update: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	// Frontend → Azure
	go func() {
		for {
			_, msg, err := s.frontend.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if err := upstream.Write(ctx, websocket.MessageText, msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Azure → Frontend (with tool call interception and text buffering).
	//
	// Text events (response.text.delta / response.text.done) are held in a
	// per-response buffer and only released to the frontend when response.done
	// arrives with no tool calls. If a tool call is intercepted, the buffer is
	// discarded — the model had started speaking before deciding to call a tool
	// and that pre-tool text should never reach the UI.
	go func() {
		var textBuf [][]byte
		responseHasTool := false

		for {
			_, msg, err := upstream.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}

			var event map[string]any
			if json.Unmarshal(msg, &event) != nil {
				_ = s.frontend.Write(ctx, websocket.MessageText, msg)
				continue
			}

			eventType, _ := event["type"].(string)

			switch eventType {
			case "response.created":
				textBuf = nil
				responseHasTool = false
				_ = s.frontend.Write(ctx, websocket.MessageText, msg)

			case "response.text.delta", "response.text.done":
				textBuf = append(textBuf, msg)

			case "response.function_call_arguments.done":
				responseHasTool = true
				textBuf = nil // discard pre-tool text
				if err := s.handleToolCall(ctx, upstream, event); err != nil {
					slog.Warn("chat/session: tool call error", "error", err)
				}

			case "response.done":
				if !responseHasTool {
					for _, b := range textBuf {
						_ = s.frontend.Write(ctx, websocket.MessageText, b)
					}
				}
				textBuf = nil
				responseHasTool = false
				_ = s.frontend.Write(ctx, websocket.MessageText, msg)

			default:
				_ = s.frontend.Write(ctx, websocket.MessageText, msg)
			}
		}
	}()

	err = <-errCh
	cancel()
	if websocket.CloseStatus(err) != -1 {
		return nil
	}
	return err
}

func (s *Session) handleToolCall(ctx context.Context, upstream *websocket.Conn, event map[string]any) error {
	callID, _ := event["call_id"].(string)
	name, _ := event["name"].(string)
	arguments, _ := event["arguments"].(string)

	result := s.tools.Dispatch(ctx, name, arguments, s.u, s.homeID)

	// Send the tool output back to Azure.
	if err := writeJSON(ctx, upstream, map[string]any{
		"type": "conversation.item.create",
		"item": map[string]any{
			"type":    "function_call_output",
			"call_id": callID,
			"output":  result,
		},
	}); err != nil {
		return fmt.Errorf("send tool output: %w", err)
	}

	// Trigger the next model response.
	return writeJSON(ctx, upstream, map[string]any{"type": "response.create"})
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, b)
}
