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

	// RabbitMQ
	RabbitMQURL string

	// Memory simulator settings
	// How many MB to allocate per second during simulation
	SimulatorMBPerSecond int

	// Environment
	Env string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8084"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "shopsre"),
		PostgresUser:     getEnv("POSTGRES_USER", "admin"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "changeme_local"),

		RabbitMQURL: getEnv(
			"RABBITMQ_URL",
			"amqp://admin:changeme_local@localhost:5672/",
		),

		// Allocate 50MB per second during simulation
		// On EKS with 256MB limit this triggers OOM in ~5 seconds
		SimulatorMBPerSecond: 50,

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