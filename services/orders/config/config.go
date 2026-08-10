package config

import "os"

type Config struct {
	// Server
	Port string

	// PostgreSQL
	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// RabbitMQ
	RabbitMQURL string

	// Auth Service
	// Orders service calls Auth service to validate JWT tokens
	AuthServiceURL string

	// Environment
	Env string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8082"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "shopsre"),
		PostgresUser:     getEnv("POSTGRES_USER", "admin"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "changeme_local"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// RabbitMQ connection URL
		// Format: amqp://user:password@host:port/
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://admin:changeme_local@localhost:5672/"),

		// Auth service URL — used to validate JWT tokens
		AuthServiceURL: getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),

		Env: getEnv("ENV", "local"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}