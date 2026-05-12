package config

import (
	"os"
	"strconv"
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

	// Certs — CA is used both to verify the broker and to issue device/backend client certs
	CACertPath string
	CAKeyPath  string

	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioUseSSL    bool
	MinioBucket    string

	// AI Service
	AIServiceURL string

	// Azure OpenAI Realtime — used by the chat package
	AzureOpenAIEndpoint string
	AzureOpenAIKey      string
	AzureDeployment     string
	AzureAPIVersion     string

	// Cohere embed-v4-0 via Azure AI Services — used for appliance embeddings
	CohereAzureEndpoint    string
	CohereAzureKey         string
	CohereAzureDeployment  string
	CohereAPIVersion       string

	// Brevo transactional email (optional — email disabled if BrevoAPIKey is empty)
	BrevoAPIKey      string
	BrevoSenderEmail string
	BrevoSenderName  string

	// FrontendURL is used to construct invite links in emails
	FrontendURL string

	// OTA safety/tuning
	OTAOnlineFreshnessSec int
	OTAAttemptTimeoutSec  int

	// PublicAPIURL is used for OTA firmware downloads
	PublicAPIURL string
	OTAAPIURL    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                  envOrDefault("PORT", "8080"),
		DatabaseURL:           envOrDefault("DATABASE_URL", "postgres://sol:sol@localhost:5432/sol?sslmode=disable"),
		RedisURL:              envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		MQTTBrokerURL: envOrDefault("MQTT_BROKER_URL", "ssl://mqtt.sol.muaathrifath.me:8883"),
		MQTTClientID:  envOrDefault("MQTT_CLIENT_ID", "sol-backend"),
		CACertPath:    os.Getenv("CA_CERT_PATH"),
		CAKeyPath:     os.Getenv("CA_KEY_PATH"),
		MinioEndpoint:         envOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:        envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:        envOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		MinioUseSSL:           os.Getenv("MINIO_USE_SSL") == "true",
		MinioBucket:           envOrDefault("MINIO_BUCKET", "firmware"),
		AIServiceURL:          envOrDefault("AI_SERVICE_URL", "http://localhost:8000"),
		AzureOpenAIEndpoint:    os.Getenv("AZURE_OPENAI_ENDPOINT"),
		AzureOpenAIKey:         os.Getenv("AZURE_OPENAI_KEY"),
		AzureDeployment:        envOrDefault("AZURE_DEPLOYMENT", "gpt-realtime-1.5"),
		AzureAPIVersion:        envOrDefault("AZURE_API_VERSION", "2025-04-01-preview"),
		CohereAzureEndpoint:    envOrDefault("COHERE_AZURE_ENDPOINT", "https://muaat-mmxdtncp-eastus2.services.ai.azure.com"),
		CohereAzureKey:         os.Getenv("COHERE_AZURE_KEY"),
		CohereAzureDeployment:  envOrDefault("COHERE_AZURE_DEPLOYMENT", "embed-v4-0"),
		CohereAPIVersion:       envOrDefault("COHERE_API_VERSION", "2024-05-01-preview"),
		BrevoAPIKey:           os.Getenv("BREVO_API_KEY"),
		BrevoSenderEmail:      envOrDefault("BREVO_SENDER_EMAIL", "noreply@sol.app"),
		BrevoSenderName:       envOrDefault("BREVO_SENDER_NAME", "Sol"),
		FrontendURL:           envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		OTAOnlineFreshnessSec: envIntOrDefault("OTA_ONLINE_FRESHNESS_SEC", 45),
		OTAAttemptTimeoutSec:  envIntOrDefault("OTA_ATTEMPT_TIMEOUT_SEC", 480),
		PublicAPIURL:          envOrDefault("PUBLIC_API_URL", "http://localhost:8080"),
		OTAAPIURL:             envOrDefault("OTA_API_URL", "https://ota.sol.muaathrifath.me"),
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
