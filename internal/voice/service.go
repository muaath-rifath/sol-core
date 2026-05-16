package voice

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
)

const (
	agentName      = "sol-voice-agent"
	tokenTTL       = 10 * time.Minute
	roomEmptyClose = 120 // seconds — room auto-closes after 2 min with no participants
)

type Config struct {
	URL       string
	APIKey    string
	APISecret string
}

// SessionInfo is published to sol/devices/{id}/voice via MQTT and returned
// from the HTTP endpoint so the ESP32 can connect to LiveKit.
type SessionInfo struct {
	RoomName string `json:"room_name"`
	Token    string `json:"token"`
	URL      string `json:"url"`
}

type mqttPublisher interface {
	Publish(topic string, payload any) error
}

type Service struct {
	cfg         Config
	roomClient  *lksdk.RoomServiceClient
	agentClient *lksdk.AgentDispatchClient
	mqtt        mqttPublisher
}

func NewService(cfg Config, mqtt mqttPublisher) *Service {
	return &Service{
		cfg:         cfg,
		roomClient:  lksdk.NewRoomServiceClient(cfg.URL, cfg.APIKey, cfg.APISecret),
		agentClient: lksdk.NewAgentDispatchServiceClient(cfg.URL, cfg.APIKey, cfg.APISecret),
		mqtt:        mqtt,
	}
}

// CreateSession creates a LiveKit room, generates an ESP32 participant token,
// and dispatches the voice agent to join. Returns the session info for the device.
func (s *Service) CreateSession(ctx context.Context, deviceID string) (*SessionInfo, error) {
	roomName := fmt.Sprintf("voice-%s-%s", deviceID, uuid.New().String()[:8])

	_, err := s.roomClient.CreateRoom(ctx, &livekit.CreateRoomRequest{
		Name:         roomName,
		EmptyTimeout: roomEmptyClose,
	})
	if err != nil {
		return nil, fmt.Errorf("create livekit room: %w", err)
	}

	token, err := s.generateToken(roomName, "esp32-"+deviceID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	// Dispatch the voice agent — non-fatal if it fails (agent may auto-dispatch)
	_, dispatchErr := s.agentClient.CreateDispatch(ctx, &livekit.CreateAgentDispatchRequest{
		AgentName: agentName,
		Room:      roomName,
	})
	if dispatchErr != nil {
		slog.Warn("voice agent dispatch failed — agent may auto-join", "room", roomName, "error", dispatchErr)
	}

	slog.Info("voice session created", "device_id", deviceID, "room", roomName)

	return &SessionInfo{
		RoomName: roomName,
		Token:    token,
		URL:      s.cfg.URL,
	}, nil
}

// HandleWake is called when MQTT receives sol/devices/{deviceId}/wake.
// It creates a session and publishes the token back to the device.
func (s *Service) HandleWake(ctx context.Context, deviceID string) {
	session, err := s.CreateSession(ctx, deviceID)
	if err != nil {
		slog.Error("voice session creation failed", "device_id", deviceID, "error", err)
		return
	}

	topic := fmt.Sprintf("sol/devices/%s/voice", deviceID)
	if err := s.mqtt.Publish(topic, session); err != nil {
		slog.Error("failed to publish voice token", "device_id", deviceID, "error", err)
	}
}

func (s *Service) generateToken(roomName, identity string) (string, error) {
	at := auth.NewAccessToken(s.cfg.APIKey, s.cfg.APISecret)
	at.SetIdentity(identity)
	at.SetValidFor(tokenTTL)
	at.AddGrant(&auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	})
	return at.ToJWT()
}
