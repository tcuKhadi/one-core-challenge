package main

import (
	"log"
	"net/http"
	"os"

	"tcuKhadi/one-core-challenge/internal/auth"
	"tcuKhadi/one-core-challenge/internal/db"
	"tcuKhadi/one-core-challenge/internal/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "challenge.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatal("database initialization failed:", err)
	}
	defer database.Close()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.HandleMethodNotAllowed = true
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})

	h := handlers.NewHandler(database)
	authMw := auth.Middleware([]byte(secret))

	r.GET("/feed", h.Feed)
	r.POST("/posts", authMw, h.CreatePost)
	r.GET("/posts/:id", h.GetPost)
	r.POST("/posts/:id/like", authMw, h.LikePost)
	r.POST("/posts/:id/comments", authMw, h.AddComment)
	r.GET("/posts/:id/comments", h.ListComments)
	r.GET("/users/:username", h.Profile)

	log.Printf("Reference server listening on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
