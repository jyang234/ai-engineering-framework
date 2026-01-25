package web

import (
	"github.com/gin-gonic/gin"

	"github.com/anthropics/aef/codex/internal/core"
)

// Server is the Codex web server
type Server struct {
	engine *core.SearchEngine
	router *gin.Engine
}

// NewServer creates a new web server
func NewServer(engine *core.SearchEngine) *Server {
	router := gin.Default()

	s := &Server{
		engine: engine,
		router: router,
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

// Run starts the web server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
