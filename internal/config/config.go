package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// MQTT
	MQTTBrokerURL string
	MQTTClientID  string

	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioUseSSL    bool
	MinioBucket    string

	// OIDC (Zitadel)
	OIDCIssuer   string
	OIDCClientID string

	// AI Service
	AIServiceURL string

	// Brevo transactional email (optional — email disabled if BrevoAPIKey is empty)
	BrevoAPIKey      string
	BrevoSenderEmail string
	BrevoSenderName  string

	// FrontendURL is used to construct invite links in emails
	FrontendURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          envOrDefault("PORT", "8080"),
		DatabaseURL:   envOrDefault("DATABASE_URL", "postgres://sol:sol@localhost:5432/sol?sslmode=disable"),
		RedisURL:      envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		MQTTBrokerURL: envOrDefault("MQTT_BROKER_URL", "tcp://localhost:1883"),
		MQTTClientID:  envOrDefault("MQTT_CLIENT_ID", "sol-backend"),
		MinioEndpoint: envOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: envOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		MinioUseSSL:   os.Getenv("MINIO_USE_SSL") == "true",
		MinioBucket:   envOrDefault("MINIO_BUCKET", "firmware"),
		OIDCIssuer:       os.Getenv("OIDC_ISSUER"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		AIServiceURL:     envOrDefault("AI_SERVICE_URL", "http://localhost:8000"),
		BrevoAPIKey:      os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail: envOrDefault("BREVO_SENDER_EMAIL", "noreply@sol.app"),
		BrevoSenderName:  envOrDefault("BREVO_SENDER_NAME", "Sol"),
		FrontendURL:      envOrDefault("FRONTEND_URL", "http://localhost:3000"),
	}

	if cfg.OIDCIssuer == "" {
		return nil, fmt.Errorf("OIDC_ISSUER is required")
	}
	if cfg.OIDCClientID == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_ID is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
