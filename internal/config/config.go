package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	InstanceID         string
	PostgresURL        string
	CassandraHosts     string
	CassandraKeyspace  string
	RedisAddr          string
	RedisPassword      string
	RabbitMQURL        string
	JWTSecret          string
	AlphabetSeed       string
	IDBatchSize        uint64
	AppBaseURL         string
	SMTPHost           string
	SMTPPort           string
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
	SMTPTLS            string
	TurnstileSiteKey   string
	TurnstileSecretKey string
	PortfolioMode      bool
}

func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		InstanceID:         getEnv("INSTANCE_ID", "api-local"),
		PostgresURL:        getEnv("POSTGRES_URL", "postgres://esmaga_user:esmaga_secret@localhost:5432/esmaga_auth?sslmode=disable"),
		CassandraHosts:     getEnv("CASSANDRA_HOSTS", "127.0.0.1"),
		CassandraKeyspace:  getEnv("CASSANDRA_KEYSPACE", "esmaga_link"),
		RedisAddr:          getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		JWTSecret:          getEnv("JWT_SECRET", "super-secret-default-key-change-in-production"),
		AlphabetSeed:       getEnv("ALPHABET_SEED", "esmaga-seed-2026"),
		IDBatchSize:        getEnvAsUint64("ID_BATCH_SIZE", 1000),
		AppBaseURL:         getEnv("APP_BASE_URL", "http://localhost:8080"),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           getEnv("SMTP_PORT", "587"),
		SMTPUser:           getEnv("SMTP_USER", ""),
		SMTPPass:           getEnv("SMTP_PASS", ""),
		SMTPFrom:           getEnv("SMTP_FROM", "Esmaga Link <noreply@esmagalink.local>"),
		SMTPTLS:            getEnv("SMTP_TLS", "starttls"),
		TurnstileSiteKey:   getEnv("TURNSTILE_SITE_KEY", ""),
		TurnstileSecretKey: getEnv("TURNSTILE_SECRET_KEY", ""),
		PortfolioMode:      getEnvAsBool("PORTFOLIO_MODE", false),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsUint64(key string, defaultVal uint64) uint64 {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseUint(valStr, 10, 64); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return defaultVal
}
