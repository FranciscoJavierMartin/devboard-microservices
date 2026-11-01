// Package main
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/FranciscoJavierMartin/devboard"
	"github.com/FranciscoJavierMartin/devboard/internal/auth"
	"github.com/FranciscoJavierMartin/devboard/internal/cache"
	"github.com/FranciscoJavierMartin/devboard/internal/handler"
	"github.com/FranciscoJavierMartin/devboard/internal/logger"
	"github.com/FranciscoJavierMartin/devboard/internal/middlewares"
	pgrepo "github.com/FranciscoJavierMartin/devboard/internal/repository/postgres"
	"github.com/FranciscoJavierMartin/devboard/internal/server"
	"github.com/FranciscoJavierMartin/devboard/internal/usecases"
	"github.com/FranciscoJavierMartin/devboard/internal/validator"
)

// @title Devboard API
// @version 1.0
// @description API REST for project management
// @host  localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Write Bearer token with format: Bearer {token}
func main() {

	logger := setupLogger()

	dbURL := os.Getenv("DATABASE_URL")
	redisAddr := os.Getenv("REDIS_ADDR")

	pool, err := pgrepo.NewPool(context.Background(), pgrepo.Config{
		URL:             dbURL,
		MaxConns:        12,
		MinConns:        5,
		MaxConnLifeTime: 1 * time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	})

	if err != nil {
		logger.Error("Could not connect to database", slog.Any("error", err))
		os.Exit(1)
	}

	defer pool.Close()

	logger.Info("Connection to database")

	redisClient, err := cache.NewClient(context.Background(), cache.Config{
		Addr:    redisAddr,
		Timeout: 5 * time.Second,
	})

	if err != nil {
		logger.Error("Could not connect to Redis instance", slog.Any("error", err))
		os.Exit(1)
	}

	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Error(
				"Could not close connection to Redis instance",
				slog.Any("error", err),
			)
		}
	}()

	jwtSecret := os.Getenv("JWT_SECRET")
	refreshTokenDuration := 7 * 24 * time.Hour

	jwtManager, err := auth.NewJWTManager(auth.JWTConfig{
		SecretKey:            []byte(jwtSecret),
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: refreshTokenDuration,
	})

	if err != nil {
		logger.Error("error setting up JWT", slog.Any("error", err))
		os.Exit(1)
	}

	tokenStore := auth.NewTokenStore(redisClient)

	// Memory
	// userRepo := memory.NewUserRepository()
	// taskRepo := memory.NewTaskRepository()

	userRepo := pgrepo.NewUserRepository(pool)
	taskRepo := pgrepo.NewTaskRepository(pool)
	workspaceRepo := pgrepo.NewWorkspaceRepository(pool)
	projectRepo := pgrepo.NewProjectRepository(pool)
	commentRepo := pgrepo.NewCommentRepository(pool)

	userUC := usecases.NewUserUseCase(userRepo)
	taskUC := usecases.NewTaskUseCase(taskRepo, taskRepo)
	authUC := usecases.NewAuthUseCase(userRepo, jwtManager, tokenStore, refreshTokenDuration)
	workspaceUC := usecases.NewWorkspaceUseCase(workspaceRepo)
	projectUC := usecases.NewProjectUseCase(projectRepo, workspaceRepo)
	commentUC := usecases.NewCommentUseCase(commentRepo, taskRepo)

	validate := validator.New()
	// notifier := notification.NewLogNotifier(log)
	// _ = notifier

	srv := server.New(
		":8080",
		server.WithLogger(logger),
		server.WithReadTimeout(15*time.Second),
		server.WithWriteTimeout(30*time.Second),
	)

	healthHandler := handler.NewHealthHandler()
	userHandler := handler.NewUserHandler(userUC, validate, logger)
	taskHandler := handler.NewTaskHandler(taskUC, validate, logger)
	authHandler := handler.NewAuthHandler(authUC, validate, logger)
	workspaceHandler := handler.NewWorkspaceHandler(workspaceUC, validate, logger)
	projectHandler := handler.NewProjectHandler(projectUC, validate, logger)
	commentHandler := handler.NewCommentHandler(commentUC, validate, logger)

	// Middleware auth
	authMW := middlewares.RequireAuth(jwtManager, logger)

	docsHandler, err := handler.NewDocsHandler(devboard.SwaggerJSON)
	if err != nil {
		logger.Error("Fatal error", "error", err)
		os.Exit(1)
	}

	srv.RegisterRoutes("GET /docs", docsHandler)
	srv.RegisterRoutes("GET /api/v1/health", healthHandler)
	// Auth routes
	srv.RegisterRoutes("POST /api/v1/auth/register", http.HandlerFunc(authHandler.Register))
	srv.RegisterRoutes("POST /api/v1/auth/login", http.HandlerFunc(authHandler.Login))
	srv.RegisterRoutes("POST /api/v1/auth/refresh", http.HandlerFunc(authHandler.Refresh))
	srv.RegisterRoutes("POST /api/v1/auth/logout", http.HandlerFunc(authHandler.Logout))
	// User routes
	srv.RegisterRoutes("POST /api/v1/users", http.HandlerFunc(userHandler.Create))
	srv.RegisterRoutes("GET /api/v1/users/{id}", http.HandlerFunc(userHandler.Get))
	// Private routes
	// Task routes
	srv.RegisterRoutes("POST /api/v1/tasks", middlewares.Chain(http.HandlerFunc(taskHandler.Create), authMW))
	srv.RegisterRoutes("GET /api/v1/tasks/{id}", middlewares.Chain(http.HandlerFunc(taskHandler.Get), authMW))
	srv.RegisterRoutes("GET /api/v1/projects/{id}/tasks", middlewares.Chain(http.HandlerFunc(taskHandler.ListByProject), authMW))
	srv.RegisterRoutes("PUT /api/v1/tasks/{id}/assign", middlewares.Chain(http.HandlerFunc(taskHandler.Assign), authMW))
	srv.RegisterRoutes("PUT /api/v1/tasks/{id}/status", middlewares.Chain(http.HandlerFunc(taskHandler.UpdateStatus), authMW))
	// Workspace routes
	srv.RegisterRoutes("POST /api/v1/workspaces", middlewares.Chain(http.HandlerFunc(workspaceHandler.Create), authMW))
	srv.RegisterRoutes("GET /api/v1/workspaces", middlewares.Chain(http.HandlerFunc(workspaceHandler.List), authMW))
	srv.RegisterRoutes("GET /api/v1/workspaces/{id}", middlewares.Chain(http.HandlerFunc(workspaceHandler.Get), authMW))
	srv.RegisterRoutes("POST /api/v1/workspaces/{id}/members", middlewares.Chain(http.HandlerFunc(workspaceHandler.AddMember), authMW))
	srv.RegisterRoutes("GET /api/v1/workspaces/{id}/members", middlewares.Chain(http.HandlerFunc(workspaceHandler.ListMembers), authMW))
	srv.RegisterRoutes("DELETE /api/v1/workspaces/{id}/members/{userID}", middlewares.Chain(http.HandlerFunc(workspaceHandler.RemoveMember), authMW))
	// Projects
	srv.RegisterRoutes("POST /api/v1/projects", middlewares.Chain(http.HandlerFunc(projectHandler.Create), authMW))
	srv.RegisterRoutes("GET /api/v1/projects/{id}", middlewares.Chain(http.HandlerFunc(projectHandler.Get), authMW))
	srv.RegisterRoutes("GET /api/v1/workspaces/{id}/projects", middlewares.Chain(http.HandlerFunc(projectHandler.ListByWorkspace), authMW))
	// Comments
	srv.RegisterRoutes("POST /api/v1/tasks/{id}/comments", middlewares.Chain(http.HandlerFunc(commentHandler.Create), authMW))
	srv.RegisterRoutes("GET /api/v1/tasks/{id}/comments", middlewares.Chain(http.HandlerFunc(commentHandler.ListByTask), authMW))
	srv.RegisterRoutes("DELETE /api/v1/comments/{id}", middlewares.Chain(http.HandlerFunc(commentHandler.Delete), authMW))

	srv.UseChain(
		middlewares.Recovery(logger),
		middlewares.Logger(logger),
		middlewares.SecurityHeaders(middlewares.SecurityConfig{
			AllowedOrigins: []string{
				"http://localhost:8080",
				"http://localhost:3000",
			},
		}),
	)

	if err := srv.Start(); err != nil {
		logger.Error("Fatal error", "error", err)
		os.Exit(1)
	}

}

func setupLogger() *slog.Logger {
	env := os.Getenv("GO_ENV")
	slog.Info("Loaded environment", "GO_ENV", env)

	var log *slog.Logger

	if env == "production" {
		log = logger.New(logger.ProductionConfig())
	} else {
		log = logger.New(logger.DefaultConfig())
	}

	return log
}
