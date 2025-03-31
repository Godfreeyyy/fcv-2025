package http

// this is entry point of the http request handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	auth2 "gitlab.com/fcv-2025.net/internal/core/services/auth"
	"gitlab.com/fcv-2025.net/internal/core/services/job"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/handlers/auth"
	"gitlab.com/fcv-2025.net/internal/handlers/jobs"
	"gitlab.com/fcv-2025.net/internal/handlers/workers"
	"gitlab.com/fcv-2025.net/internal/tcp"
)

type ServiceProvider struct {
	workerService worker.IWorkerRegistrationService
	jobService    job.IJobService

	ggAuth    auth2.IAuthService
	localAuth auth2.IAuthService
}

func NewServiceProvider(
	workerService worker.IWorkerRegistrationService,
	jobService job.IJobService,
	ggAuth auth2.IAuthService,
	localAuth auth2.IAuthService,
) *ServiceProvider {
	return &ServiceProvider{
		workerService: workerService,
		jobService:    jobService,
		ggAuth:        ggAuth,
		localAuth:     localAuth,
	}
}

type Server struct {
	router          *mux.Router
	Port            int
	ServiceName     string
	ServiceProvider ServiceProvider
	logger          primary.Logger
	tcpServer       *tcp.TCPServer
}

func NewServer(port int, serviceName string, serviceProvider ServiceProvider, tcpServer *tcp.TCPServer, logger primary.Logger) *Server {
	return &Server{
		Port:            port,
		ServiceName:     serviceName,
		ServiceProvider: serviceProvider,
		logger:          logger,
		tcpServer:       tcpServer,
	}
}

func (s *Server) Init() error {
	r := mux.NewRouter()
	workers.NewHandler(s.ServiceProvider.workerService).Register(r)
	jobs.
		NewJobHandler(s.ServiceProvider.jobService, s.logger).
		RegisterRoutes(r, s.tcpServer)
	auth.NewHandler().RegisterRoutes(r, &auth.ServiceDependencies{
		GGAuthService:    s.ServiceProvider.ggAuth,
		LocalAuthService: s.ServiceProvider.localAuth,
	})
	s.router = r
	return nil
}

func (s *Server) Start(ctx context.Context) {
	// Set up server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.Port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		s.logger.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

}

func (s *Server) Stop() {
	s.logger.Info("Shutting down http server...")
}
