package config

import (
	"os"
)

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

	// JWT
	JWTSecret string

	// Environment
	Env string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8081"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "shopsre"),
		PostgresUser:     getEnv("POSTGRES_USER", "admin"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "changeme_local"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		JWTSecret: getEnv("JWT_SECRET", "local_dev_secret_32_chars_minimum"),

		Env: getEnv("ENV", "local"),
	}
}

// getEnv reads an environment variable.
// If it is not set, it returns the fallback value.
func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}