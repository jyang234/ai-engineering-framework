package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/aef/codex/internal/core"
)

// Web handlers

func (s *Server) handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Codex - Knowledge Base",
	})
}

func (s *Server) handleSearch(c *gin.Context) {
	query := c.Query("q")
	types := c.QueryArray("type")

	if query == "" {
		c.HTML(http.StatusOK, "search.html", gin.H{
			"query":   "",
			"results": nil,
			"count":   0,
		})
		return
	}

	results, err := s.engine.Search(c.Request.Context(), core.SearchRequest{
		Query: query,
		Types: types,
		Limit: 20,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "search.html", gin.H{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleItem(c *gin.Context) {
	id := c.Param("id")

	item, err := s.engine.Get(c.Request.Context(), id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Item not found"})
		return
	}

	c.HTML(http.StatusOK, "item.html", gin.H{
		"item": item,
	})
}

func (s *Server) handleBrowse(c *gin.Context) {
	itemType := c.Query("type")
	scope := c.Query("scope")

	// TODO: Implement browsing with pagination
	c.HTML(http.StatusOK, "browse.html", gin.H{
		"type":  itemType,
		"scope": scope,
	})
}

// API handlers

func (s *Server) handleAPISearch(c *gin.Context) {
	query := c.Query("q")
	types := c.QueryArray("type")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter required"})
		return
	}

	results, err := s.engine.Search(c.Request.Context(), core.SearchRequest{
		Query: query,
		Types: types,
		Limit: 20,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleAPIItem(c *gin.Context) {
	id := c.Param("id")

	item, err := s.engine.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

func (s *Server) handleAPICreate(c *gin.Context) {
	var item core.Item
	if err := c.BindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.engine.Add(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      item.ID,
		"message": "Item created",
	})
}

func (s *Server) handleAPIUpdate(c *gin.Context) {
	// TODO: Implement update
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

func (s *Server) handleAPIDelete(c *gin.Context) {
	// TODO: Implement delete
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
