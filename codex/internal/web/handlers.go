package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/storage"
)

const (
	maxContentSize = 1 << 20 // 1MB
	maxQuerySize   = 10 << 10 // 10KB
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
	pageStr := c.DefaultQuery("page", "1")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limit := 50
	offset := (page - 1) * limit

	items, err := s.engine.List(c.Request.Context(), itemType, scope, limit, offset)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "browse.html", gin.H{
		"type":  itemType,
		"scope": scope,
		"items": items,
		"count": len(items),
		"page":  page,
	})
}

// API handlers

func (s *Server) handleAPISearch(c *gin.Context) {
	query := c.Query("q")
	types := c.QueryArray("type")

	if len(query) > maxQuerySize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "query exceeds maximum size of 10KB",
		})
		return
	}

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "query parameter required",
		})
		return
	}

	results, err := s.engine.Search(c.Request.Context(), core.SearchRequest{
		Query: query,
		Types: types,
		Limit: 20,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) handleAPIItem(c *gin.Context) {
	id := c.Param("id")

	item, err := s.engine.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "item not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    item,
	})
}

func (s *Server) handleAPICreate(c *gin.Context) {
	var item core.Item
	if err := c.BindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if len(item.Content) > maxContentSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "content exceeds maximum size of 1MB",
		})
		return
	}

	// Set timestamps and ID if not provided
	now := time.Now()
	if item.ID == "" {
		item.ID = storage.GenerateID()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	// Default scope
	if item.Scope == "" {
		item.Scope = "project"
	}

	if err := s.engine.Add(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      item.ID,
		"message": "Item created",
	})
}

func (s *Server) handleAPIUpdate(c *gin.Context) {
	id := c.Param("id")

	var item core.Item
	if err := c.BindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Ensure ID matches
	item.ID = id

	if len(item.Content) > maxContentSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "content exceeds maximum size of 1MB",
		})
		return
	}

	if err := s.engine.Update(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      item.ID,
		"message": "Item updated",
	})
}

func (s *Server) handleAPIDelete(c *gin.Context) {
	id := c.Param("id")

	if err := s.engine.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item deleted",
	})
}
