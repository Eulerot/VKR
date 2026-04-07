package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TCPPort    string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMaxConns int32
}

func Load() Config {
	return Config{
		TCPPort:    getEnv("TCP_PORT", "8080"),
		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "planner_user"),
		DBPassword: getEnv("DB_PASSWORD", "strong_password_123"),
		DBName:     getEnv("DB_NAME", "repair_planner"),
		DBMaxConns: int32(getEnvInt("DB_MAX_CONNS", 4)),
	}
}

func (c Config) ConnString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}