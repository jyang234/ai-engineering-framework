package main

import (
	"encoding/json"
	"net/http"

	"github.com/anthropics/aef/codex/eval"
	"github.com/gin-gonic/gin"
)

func main() {
	collection := eval.NewPayFlowCollection()

	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"documents": len(collection.Documents),
			"queries":   len(collection.Queries),
		})
	})

	r.GET("/documents", func(c *gin.Context) {
		c.JSON(http.StatusOK, collection.Documents)
	})

	r.GET("/documents/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, doc := range collection.Documents {
			if doc.ID == id {
				c.JSON(http.StatusOK, doc)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	r.GET("/queries", func(c *gin.Context) {
		c.JSON(http.StatusOK, collection.Queries)
	})

	r.GET("/queries/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, q := range collection.Queries {
			if q.ID == id {
				c.JSON(http.StatusOK, q)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	r.GET("/export", func(c *gin.Context) {
		data, _ := json.MarshalIndent(collection, "", "  ")
		c.Data(http.StatusOK, "application/json", data)
	})

	r.Run(":8088")
}
