package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tcuKhadi/one-core-challenge/internal/models"

	"github.com/gin-gonic/gin"
)

const postSelect = `
	SELECT p.id, p.username, p.content, p.created_at,
	       (SELECT COUNT(*) FROM likes l WHERE l.post_id = p.id),
	       (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id)
	FROM posts p`

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func validContent(c string) bool {
	n := utf8.RuneCountInString(c)
	return n >= 1 && n <= 280 && strings.TrimSpace(c) != ""
}

func parsePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return 0, false
	}
	return id, true
}

func (h *Handler) postExists(id int64) bool {
	var one int
	return h.db.QueryRow(`SELECT 1 FROM posts WHERE id = ?`, id).Scan(&one) == nil
}

// 1. GET /feed
func (h *Handler) Feed(c *gin.Context) {
	rows, err := h.db.Query(postSelect + ` ORDER BY p.id DESC LIMIT 20`)
	if err != nil {
		log.Println("feed:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Username, &p.Content, &p.CreatedAt, &p.Likes, &p.Comments); err != nil {
			log.Println("feed scan:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		posts = append(posts, p)
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// 2. POST /posts
func (h *Handler) CreatePost(c *gin.Context) {
	var b models.ContentReq
	if err := c.ShouldBindJSON(&b); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if !validContent(b.Content) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "content must be 1-280 unicode characters and not whitespace"})
		return
	}

	user := c.GetString("username")
	tx, err := h.db.Begin()
	if err != nil {
		log.Println("createPost begin:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer tx.Rollback()

	createdAt := nowRFC3339()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO users (username) VALUES (?)`, user); err != nil {
		log.Println("createPost user:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	res, err := tx.Exec(`INSERT INTO posts (username, content, created_at) VALUES (?, ?, ?)`, user, b.Content, createdAt)
	if err != nil {
		log.Println("createPost insert:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Println("createPost id:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Println("createPost commit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"post": models.Post{
			ID:        id,
			Username:  user,
			Content:   b.Content,
			CreatedAt: createdAt,
			Likes:     0,
			Comments:  0,
		},
	})
}

// 3. GET /posts/:id
func (h *Handler) GetPost(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}

	var p models.Post
	err := h.db.QueryRow(postSelect+` WHERE p.id = ?`, id).
		Scan(&p.ID, &p.Username, &p.Content, &p.CreatedAt, &p.Likes, &p.Comments)
	if err == sql.ErrNoRows {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	if err != nil {
		log.Println("getPost:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": p})
}

// 4. POST /posts/:id/like
func (h *Handler) LikePost(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}

	user := c.GetString("username")
	tx, err := h.db.Begin()
	if err != nil {
		log.Println("likePost begin:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer tx.Rollback()

	var one int
	if err := tx.QueryRow(`SELECT 1 FROM posts WHERE id = ?`, id).Scan(&one); err == sql.ErrNoRows {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	} else if err != nil {
		log.Println("likePost lookup:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO users (username) VALUES (?)`, user); err != nil {
		log.Println("likePost user:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	res, err := tx.Exec(`INSERT OR IGNORE INTO likes (post_id, username) VALUES (?, ?)`, id, user)
	if err != nil {
		log.Println("likePost insert:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	added, err := res.RowsAffected()
	if err != nil {
		log.Println("likePost rows:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var total int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM likes WHERE post_id = ?`, id).Scan(&total); err != nil {
		log.Println("likePost count:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Println("likePost commit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	status := http.StatusOK
	if added == 1 {
		status = http.StatusCreated
	}

	c.JSON(status, gin.H{"post_id": id, "likes": total})
}

// 5. POST /posts/:id/comments
func (h *Handler) AddComment(c *gin.Context) {
	var b models.ContentReq
	if err := c.ShouldBindJSON(&b); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if !validContent(b.Content) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "content must be 1-280 characters and not only whitespace"})
		return
	}

	id, ok := parsePathID(c)
	if !ok {
		return
	}

	user := c.GetString("username")
	tx, err := h.db.Begin()
	if err != nil {
		log.Println("addComment begin:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer tx.Rollback()

	var one int
	if err := tx.QueryRow(`SELECT 1 FROM posts WHERE id = ?`, id).Scan(&one); err == sql.ErrNoRows {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	} else if err != nil {
		log.Println("addComment lookup:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	createdAt := nowRFC3339()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO users (username) VALUES (?)`, user); err != nil {
		log.Println("addComment user:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if _, err := tx.Exec(`INSERT INTO comments (post_id, username, content, created_at) VALUES (?, ?, ?, ?)`,
		id, user, b.Content, createdAt); err != nil {
		log.Println("addComment insert:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Println("addComment commit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"comment": models.Comment{
			PostID:    id,
			Username:  user,
			Content:   b.Content,
			CreatedAt: createdAt,
		},
	})
}

// 6. GET /posts/:id/comments
func (h *Handler) ListComments(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}

	if !h.postExists(id) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	rows, err := h.db.Query(`
		SELECT post_id, username, content, created_at
		FROM comments
		WHERE post_id = ?
		ORDER BY id DESC
		LIMIT 20`, id)
	if err != nil {
		log.Println("listComments:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer rows.Close()

	list := make([]models.Comment, 0)
	for rows.Next() {
		var cm models.Comment
		if err := rows.Scan(&cm.PostID, &cm.Username, &cm.Content, &cm.CreatedAt); err != nil {
			log.Println("listComments scan:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		list = append(list, cm)
	}

	c.JSON(http.StatusOK, gin.H{"comments": list})
}

// 7. GET /users/:username
func (h *Handler) Profile(c *gin.Context) {
	u := c.Param("username")
	tx, err := h.db.Begin()
	if err != nil {
		log.Println("profile begin:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer tx.Rollback()

	var visits int64
	err = tx.QueryRow(`
		UPDATE users
		SET profile_visits = profile_visits + 1
		WHERE username = ?
		RETURNING profile_visits`, u).Scan(&visits)
	if err == sql.ErrNoRows {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		log.Println("profile visit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var postCount int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM posts WHERE username = ?`, u).Scan(&postCount); err != nil {
		log.Println("profile posts:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Println("profile commit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": models.UserProfile{
			Username:      u,
			PostCount:     postCount,
			ProfileVisits: visits,
		},
	})
}
