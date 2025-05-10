package main

import (
	"context"
	"forum_service/internal/config"
	"forum_service/internal/repo"
	"forum_service/internal/service"
	DB "forum_service/pkg/db"

	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var err error

	// Инициализация логера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Загрузка конфигурации
	cfg, err := config.LoadConfig("config.ini")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Инициализация базы данных
	db, err := DB.NewDB(cfg)
	if err != nil {
		logger.Error("Failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close db connection", "error", err)
		}
	}()

	// Инициализация Redis (rdb)
	rdb, err := DB.NewRedis(cfg)
	if err != nil {
		logger.Error("Failed to connect to rdb", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("Failed to close rdb connection", "error", err)
		}
	}()

	// Инициализация репозиториев и сервисов
	repository := repo.NewForumRepo(db, rdb)
	forumService, err := service.NewForumService(
		repository,
		cfg.Connections.Services.Users,
		cfg.Connections.Services.Config,
		cfg.Connections.Services.TimeOut,
	)
	if err != nil {
		logger.Error("Failed to create forum service", "error", err)
		os.Exit(1)
	}

	// Инициализация и запуск сервера
	server, err := NewServer(cfg, forumService)
	if err != nil {
		logger.Error("Failed to create server", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			logger.Error("Server error", "error", err)
			done <- syscall.SIGTERM
		}
	}()

	logger.Info("Forum service started", "port", cfg.Server.Port)

	<-done
	logger.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	} else {
		logger.Info("Server stopped gracefully")
	}
}
