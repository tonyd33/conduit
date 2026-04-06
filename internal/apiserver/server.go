package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// _ "github.com/tonyd33/conduit/docs" // Import generated docs
)

// @title Conduit API
// @version 1.0
// @description API server for managing Conduit exchanges
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url https://github.com/tonyd33/conduit
// @contact.email support@conduit.example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8090
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and your API key.

// Server represents the Conduit API server
type Server struct {
	client  client.Client
	natsURL string
	addr    string
	apiKey  string // Simple API key authentication
	server  *http.Server
}

// Config holds the API server configuration
type Config struct {
	// Address to bind the API server to (e.g., ":8090")
	Address string

	// NATS URL to provide to clients
	NATSURL string

	// API key for authentication (optional, if empty auth is disabled)
	APIKey string
}

// NewServer creates a new API server
func NewServer(k8sClient client.Client, config *Config) *Server {
	s := &Server{
		client:  k8sClient,
		natsURL: config.NATSURL,
		addr:    config.Address,
		apiKey:  config.APIKey,
	}

	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/api/v1/exchanges", s.authMiddleware(s.handleExchanges))
	mux.HandleFunc("/api/v1/exchanges/", s.authMiddleware(s.handleExchange))
	mux.HandleFunc("/healthz", s.handleHealth)

	// Swagger UI
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	s.server = &http.Server{
		Addr:         config.Address,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the API server (implements controller-runtime Runnable interface)
func (s *Server) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("api-server")
	logger.Info("Starting API server", "address", s.addr)

	errCh := make(chan error, 1)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("Shutting down API server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// NeedLeaderElection returns false - API server doesn't require leader election
func (s *Server) NeedLeaderElection() bool {
	return false
}

// authMiddleware checks for API key authentication
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no API key configured, skip authentication
		if s.apiKey == "" {
			next(w, r)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + s.apiKey

		if authHeader != expectedAuth {
			s.writeError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}

		next(w, r)
	}
}

// handleExchanges handles requests to /api/v1/exchanges
func (s *Server) handleExchanges(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listExchanges(w, r)
	case http.MethodPost:
		s.createExchange(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleExchange handles requests to /api/v1/exchanges/{namespace}/{name}
func (s *Server) handleExchange(w http.ResponseWriter, r *http.Request) {
	// Parse namespace and name from path
	// Path format: /api/v1/exchanges/{namespace}/{name}
	path := r.URL.Path[len("/api/v1/exchanges/"):]

	switch r.Method {
	case http.MethodGet:
		s.getExchange(w, r, path)
	case http.MethodDelete:
		s.deleteExchange(w, r, path)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// listExchanges lists all Exchanges
// @Summary List exchanges
// @Description Get a list of all exchanges, optionally filtered by namespace
// @Tags exchanges
// @Accept json
// @Produce json
// @Param namespace query string false "Namespace to filter exchanges"
// @Success 200 {object} ListExchangesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /exchanges [get]
func (s *Server) listExchanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get namespace from query parameter (optional)
	namespace := r.URL.Query().Get("namespace")

	exchangeList := &conduitv1alpha1.ExchangeList{}
	listOpts := []client.ListOption{}

	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := s.client.List(ctx, exchangeList, listOpts...); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list exchanges: %v", err))
		return
	}

	// Convert to API response
	exchanges := make([]ExchangeResponse, len(exchangeList.Items))
	for i, exchange := range exchangeList.Items {
		exchanges[i] = *FromExchange(&exchange, s.natsURL)
	}

	response := ListExchangesResponse{
		Exchanges: exchanges,
		Total:     len(exchanges),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// createExchange creates a new Exchange
// @Summary Create an exchange
// @Description Create a new exchange with the specified configuration
// @Tags exchanges
// @Accept json
// @Produce json
// @Param exchange body CreateExchangeRequest true "Exchange configuration"
// @Success 201 {object} ExchangeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /exchanges [post]
func (s *Server) createExchange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.FromContext(ctx)

	var req CreateExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Validate request
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Set defaults
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	// Create Exchange CR
	exchange := &conduitv1alpha1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: *req.ToExchangeSpec(),
	}

	if err := s.client.Create(ctx, exchange); err != nil {
		logger.Error(err, "Failed to create Exchange", "name", req.Name, "namespace", req.Namespace)
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create exchange: %v", err))
		return
	}

	logger.Info("Created Exchange via API", "name", req.Name, "namespace", req.Namespace)

	// Return response
	response := FromExchange(exchange, s.natsURL)
	s.writeJSON(w, http.StatusCreated, response)
}

// getExchange gets a specific Exchange
// @Summary Get an exchange
// @Description Get details of a specific exchange by namespace and name
// @Tags exchanges
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace"
// @Param name path string true "Exchange name"
// @Success 200 {object} ExchangeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /exchanges/{namespace}/{name} [get]
func (s *Server) getExchange(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()

	// Parse namespace and name
	// Expected format: {namespace}/{name}
	namespace, name := parseNamespacedName(path)
	if namespace == "" || name == "" {
		s.writeError(w, http.StatusBadRequest, "Invalid path format, expected: /api/v1/exchanges/{namespace}/{name}")
		return
	}

	exchange := &conduitv1alpha1.Exchange{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, exchange); err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("Exchange not found: %v", err))
		return
	}

	response := FromExchange(exchange, s.natsURL)
	s.writeJSON(w, http.StatusOK, response)
}

// deleteExchange deletes a specific Exchange
// @Summary Delete an exchange
// @Description Delete a specific exchange by namespace and name
// @Tags exchanges
// @Accept json
// @Produce json
// @Param namespace path string true "Namespace"
// @Param name path string true "Exchange name"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /exchanges/{namespace}/{name} [delete]
func (s *Server) deleteExchange(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	logger := log.FromContext(ctx)

	// Parse namespace and name
	namespace, name := parseNamespacedName(path)
	if namespace == "" || name == "" {
		s.writeError(w, http.StatusBadRequest, "Invalid path format, expected: /api/v1/exchanges/{namespace}/{name}")
		return
	}

	exchange := &conduitv1alpha1.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := s.client.Delete(ctx, exchange); err != nil {
		logger.Error(err, "Failed to delete Exchange", "name", name, "namespace", namespace)
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete exchange: %v", err))
		return
	}

	logger.Info("Deleted Exchange via API", "name", name, "namespace", namespace)

	w.WriteHeader(http.StatusNoContent)
}

// handleHealth handles health check requests
// @Summary Health check
// @Description Check if the API server is healthy
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Helper functions

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, ErrorResponse{
		Error: message,
		Code:  status,
	})
}

// parseNamespacedName parses a path like "default/my-exchange" into namespace and name
func parseNamespacedName(path string) (namespace, name string) {
	// Simple parsing - look for first slash
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			return path[:i], path[i+1:]
		}
	}
	// No slash found, treat entire path as name in default namespace
	return "default", path
}
