package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"repairplanner/internal/db"
	"repairplanner/internal/repository"
	"repairplanner/internal/server"
)

type config struct {
	TCPPort    string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMaxConns int32
}

func loadConfig() config {
	return config{
		TCPPort:    getenv("TCP_PORT", "8080"),
		DBHost:     getenv("DB_HOST", "postgres"),
		DBPort:     getenv("DB_PORT", "5432"),
		DBUser:     getenv("DB_USER", "planner_user"),
		DBPassword: getenv("DB_PASSWORD", "strong_password_123"),
		DBName:     getenv("DB_NAME", "repair_planner"),
		DBMaxConns: 4,
	}
}

func (c config) ConnString() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=disable"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.ConnString(), cfg.DBMaxConns)
	if err != nil {
		log.Fatal("db pool: ", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("db ping: ", err)
	}

	repo := repository.New(pool)
	srv := server.New(repo)

	ln, err := net.Listen("tcp", ":"+cfg.TCPPort)
	if err != nil {
		log.Fatal("listen: ", err)
	}
	defer ln.Close()

	log.Printf("tcp server started on :%s", cfg.TCPPort)

	if err := srv.Serve(ctx, ln); err != nil {
		log.Fatal("server: ", err)
	}
}