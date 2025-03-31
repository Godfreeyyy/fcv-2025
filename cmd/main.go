package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"gitlab.com/fcv-2025.net/internal/adapter/crypto"
	"gitlab.com/fcv-2025.net/internal/adapter/logging"
	"gitlab.com/fcv-2025.net/internal/adapter/postgres/jobconfig"
	"gitlab.com/fcv-2025.net/internal/adapter/postgres/jobrepository"
	"gitlab.com/fcv-2025.net/internal/adapter/postgres/userrepository"
	"gitlab.com/fcv-2025.net/internal/adapter/redis/workerport"
	"gitlab.com/fcv-2025.net/internal/config"
	auth2 "gitlab.com/fcv-2025.net/internal/core/services/auth"
	"gitlab.com/fcv-2025.net/internal/core/services/job"
	"gitlab.com/fcv-2025.net/internal/core/services/schedule"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	logger2 "gitlab.com/fcv-2025.net/internal/global/logger"
	"gitlab.com/fcv-2025.net/internal/handlers"
	http2 "gitlab.com/fcv-2025.net/internal/http"
	"gitlab.com/fcv-2025.net/internal/schedulerengine"
	"gitlab.com/fcv-2025.net/internal/tcp"
)

func main() {
	InitReader()
	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	logger2.Info("Starting job scheduler service")

	logger := logger2.Logger

	sysCfg := config.NewSystemConfig()

	db, err := setupDatabase()
	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     sysCfg.RedisConfig.Url,
		Password: sysCfg.RedisConfig.Password, // no password set
		DB:       sysCfg.RedisConfig.DB,       // use default DB
	})

	// SECONDARY PORTS
	workerPort := workerport.NewWorkerRepository(redisClient, logger)
	workerService := worker.NewWorkerRegistrationService(workerPort, logger)
	jobRepo := jobrepository.NewJobRepository(db, logger)
	schedulerPort := schedule.NewSchedulerService(jobRepo, workerPort, logger, sysCfg.ScheduleSvcCfg)
	jobTtypeConfigRepo := jobconfig.NewJobTypeConfigRepository(db, logger)
	jobPort := jobrepository.NewJobRepository(db, logger)
	userPort := userrepository.New(db, logger, "public")

	//primary ports
	jwtProvider := crypto.NewJWTService(sysCfg.JwtConfig)

	//services
	jobSvc := job.NewJobService(jobPort, jobTtypeConfigRepo, logger)
	ggAuth := auth2.NewGoogleAuthService(userPort, jwtProvider, sysCfg.GGAuthConfig)
	localAuth := auth2.NewLocalAuthService(userPort, jwtProvider)
	serviceProvider := http2.NewServiceProvider(workerService, jobSvc, ggAuth, localAuth)

	//server
	tcpServer := tcp.NewTCPServer(workerService, schedulerPort, jobSvc, logging.NewZapLogger(), tcp.WithAddress(":8080"))
	httServer := http2.NewServer(8082, "jobScheduler", *serviceProvider, tcpServer, logger)
	err = httServer.Init()
	if err != nil {
		panic(err)
	}
	ctxBg := context.Background()
	httServer.Start(ctxBg)
	tcpServer.Start()
	schedulerSvc := schedulerengine.NewSchedulerEngine(sysCfg.ScheduleSvcCfg, jobPort, logger, tcpServer)
	if !sysCfg.DebugMode {
		schedulerSvc.StartJobScheduleEngine(ctxBg)
	}
	//startSchedulerTasks(schedulerPort, *logger)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cacel := context.WithTimeout(ctxBg, 5*time.Second)
	defer cacel()
	tcpServer.Stop(ctx)
	httServer.Stop()

	logger.Info("successfully shutdown server")

}

func httpServer() {
	logger := logging.NewZapLogger()

	// Initialize logger
	logger.Info("Starting job scheduler service")

	// Set up dependencies
	db, err := setupDatabase()
	if err != nil {
		logger.Error("Failed to set up database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient := setupRedis()
	defer redisClient.Close()

	// Create repositories
	jobRepo := jobrepository.NewJobRepository(db, logger)
	workerRepo := workerport.NewWorkerRepository(redisClient, logger)
	//jobConfigRepo := jobconfig.NewJobTypeConfigRepository(db, logger)

	// Create services
	//jobService := job.NewJobService(jobRepo, jobConfigRepo, logger)
	schedulerService := schedule.NewSchedulerService(jobRepo, workerRepo, logger, nil)
	workerService := worker.NewWorkerRegistrationService(workerRepo, logger)

	// Start scheduler background tasks
	startSchedulerTasks(schedulerService, *logger)

	// Set up HTTP server
	router := mux.NewRouter()

	router.HandleFunc("/workers", func(writer http.ResponseWriter, request *http.Request) {

	})

	workerHandler := handlers.NewWorkerHandler(workerService, logger)
	workerHandler.RegisterRoutes(router)

	// Set up server
	srv := &http.Server{
		Addr:         getEnv("SERVER_ADDR", ":8080"),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited")
}

// setupDatbase sets up the PostgreSQL connection
func setupDatabase() (*sqlx.DB, error) {
	connStr := getEnv("DATABASE_URL", "postgres://root:123456@localhost:5432/postgres?sslmode=disable")
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// setupRedis sets up the Redis connection
func setupRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
}

// getEnv gets an environment variable with a fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// startSchedulerTasks starts the scheduler background tasks
func startSchedulerTasks(scheduler schedule.ISchedulerService, logger logging.ZapLogger) {
	// Task to schedule pending jobs
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			ctx := context.Background()
			if err := scheduler.ScheduleJobs(ctx); err != nil {
				logger.Error("Failed to schedule jobs", "error", err)
				continue
			}

		}
	}()

	// Task to assign jobs to workers
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			ctx := context.Background()
			if err := scheduler.AssignJobs(ctx); err != nil {
				logger.Error("Failed to assign jobs", "error", err)
			}
		}
	}()
}

// getIntEnv gets an environment variable as an integer with a fallback
func getIntEnv(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := fmt.Sscanf(value, "%d"); err == nil {
			return intValue
		}
	}
	return fallback
}

func InitReader() {
	environment := ""
	if len(os.Args) < 2 {
		log.Fatalf("Env not supplied in argument")
	} else {
		environment = os.Args[1]
	}

	err := godotenv.Load(environment + ".env")
	if err != nil {
		log.Fatalf("Error loading %s.env file", environment)
	}
}
