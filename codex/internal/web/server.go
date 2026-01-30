package web

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/aef/codex/internal/core"
)

// ServerConfig holds web server configuration
type ServerConfig struct {
	APIKey string // If set, require Bearer token auth on all routes
}

// Server is the Codex web server
type Server struct {
	engine *core.SearchEngine
	router *gin.Engine
	config ServerConfig
}

// NewServer creates a new web server
func NewServer(engine *core.SearchEngine, opts ...ServerOption) *Server {
	router := gin.Default()

	s := &Server{
		engine: engine,
		router: router,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Set up template functions
	router.SetFuncMap(template.FuncMap{
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"slice": func(s string, start, end int) string {
			if len(s) <= end {
				return s
			}
			return s[start:end]
		},
	})

	// Apply auth middleware if API key is configured
	if s.config.APIKey != "" {
		router.Use(s.apiKeyAuth())
	}

	// Load templates
	router.LoadHTMLGlob("web/templates/*")
	router.Static("/static", "web/static")

	// Web routes
	router.GET("/", s.handleIndex)
	router.GET("/search", s.handleSearch)
	router.GET("/item/:id", s.handleItem)
	router.GET("/browse", s.handleBrowse)

	// API routes
	api := router.Group("/api")
	{
		api.GET("/search", s.handleAPISearch)
		api.GET("/item/:id", s.handleAPIItem)
		api.POST("/item", s.handleAPICreate)
		api.PUT("/item/:id", s.handleAPIUpdate)
		api.DELETE("/item/:id", s.handleAPIDelete)
	}

	return s
}

// ServerOption configures a Server
type ServerOption func(*Server)

// WithAPIKey sets the API key for bearer token authentication
func WithAPIKey(key string) ServerOption {
	return func(s *Server) {
		s.config.APIKey = key
	}
}

// apiKeyAuth returns middleware that checks for a valid Bearer token
func (s *Server) apiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.config.APIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized: invalid or missing API key",
			})
			return
		}
		c.Next()
	}
}

// Run starts the web server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
