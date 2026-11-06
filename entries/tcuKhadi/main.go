// one-core-challenge reference solution - Go + Gin + SQLite
// Implements api.md. Not trying to push limits: simple, correct, and
// durability-first. storage (rules.md): WAL journal + synchronous=FULL,
// so every completed write is fsynced before the reply goes out.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite" // pure-Go driver, no cgo
)

var (
	db         *sql.DB
	usernameRe = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

const schema = `
CREATE TABLE IF NOT EXISTS posts (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	username   TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS likes (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS comments (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id    INTEGER NOT NULL,
	username   TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
	username       TEXT PRIMARY KEY,
	profile_visits INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_likes_post    ON likes(post_id);
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id);
CREATE INDEX IF NOT EXISTS idx_posts_user    ON posts(username);
`

type comment struct {
	PostID    int64  `json:"post_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- validation -----------------------------------------------------------

func validUsername(u string) bool { return usernameRe.MatchString(u) }

func validContent(c string) bool {
	n := utf8.RuneCountInString(c)
	return n >= 1 && n <= 280
}

func badRequest(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": msg})
}

func postNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
}

func parsePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		postNotFound(c)
		return 0, false
	}
	return id, true
}

// ---- 1. GET /feed ---------------------------------------------------------

func feed(c *gin.Context) {
	rows, err := db.Query(`
		SELECT p.id, p.username, p.content, p.created_at,
		       (SELECT COUNT(*) FROM likes l    WHERE l.post_id = p.id),
		       (SELECT COUNT(*) FROM comments w WHERE w.post_id = p.id)
		FROM posts p
		ORDER BY p.id DESC`) // newest first; ids increase with time
	if err != nil {
		log.Println("feed:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer rows.Close()

	posts := []gin.H{}
	for rows.Next() {
		var id, likes, comments int64
		var u, content, createdAt string
		if err := rows.Scan(&id, &u, &content, &createdAt, &likes, &comments); err != nil {
			log.Println("feed scan:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		posts = append(posts, gin.H{
			"id": id, "username": u, "content": content,
			"likes": likes, "comments": comments, "created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// ---- 2. POST /posts --------------------------------------------------------

type createPostReq struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func createPost(c *gin.Context) {
	var b createPostReq
	if err := c.ShouldBindJSON(&b); err != nil {
		badRequest(c, "invalid JSON body")
		return
	}
	if !validUsername(b.Username) {
		badRequest(c, "username must be 1-32 chars: lowercase letters, numbers, underscores")
		return
	}
	if !validContent(b.Content) {
		badRequest(c, "content must be 1-280 characters")
		return
	}

	createdAt := nowRFC3339()
	res, err := db.Exec(`INSERT INTO posts (username, content, created_at) VALUES (?, ?, ?)`,
		b.Username, b.Content, createdAt)
	if err != nil {
		log.Println("createPost:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// The insert committed (fsynced per synchronous=FULL) before the reply.
	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"id": id, "username": b.Username, "content": b.Content,
		"likes": 0, "comments": 0, "created_at": createdAt,
	})
}

// ---- 3. GET /posts/:id -----------------------------------------------------

func postExists(id int64) bool {
	var one int
	err := db.QueryRow(`SELECT 1 FROM posts WHERE id = ?`, id).Scan(&one)
	return err == nil
}

func getPost(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var u, content, createdAt string
	var likes, comments int64
	err := db.QueryRow(`
		SELECT p.username, p.content, p.created_at,
		       (SELECT COUNT(*) FROM likes l    WHERE l.post_id = p.id),
		       (SELECT COUNT(*) FROM comments w WHERE w.post_id = p.id)
		FROM posts p WHERE p.id = ?`, id).
		Scan(&u, &content, &createdAt, &likes, &comments)
	if err == sql.ErrNoRows {
		postNotFound(c)
		return
	}
	if err != nil {
		log.Println("getPost:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": id, "username": u, "content": content,
		"likes": likes, "comments": comments, "created_at": createdAt,
	})
}

// ---- 4. POST /posts/:id/like ------------------------------------------------

func likePost(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if !postExists(id) {
		postNotFound(c)
		return
	}
	// One request adds exactly one like. Liking twice counts twice.
	if _, err := db.Exec(`INSERT INTO likes (post_id) VALUES (?)`, id); err != nil {
		log.Println("likePost:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM likes WHERE post_id = ?`, id).Scan(&total); err != nil {
		log.Println("likePost count:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"post_id": id, "likes": total})
}

// ---- 5. POST /posts/:id/comments ---------------------------------------------

type addCommentReq struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func addComment(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var b addCommentReq
	if err := c.ShouldBindJSON(&b); err != nil {
		badRequest(c, "invalid JSON body")
		return
	}
	if !validUsername(b.Username) {
		badRequest(c, "username must be 1-32 chars: lowercase letters, numbers, underscores")
		return
	}
	if !validContent(b.Content) {
		badRequest(c, "content must be 1-280 characters")
		return
	}
	if !postExists(id) {
		postNotFound(c)
		return
	}

	createdAt := nowRFC3339()
	if _, err := db.Exec(`INSERT INTO comments (post_id, username, content, created_at) VALUES (?, ?, ?, ?)`,
		id, b.Username, b.Content, createdAt); err != nil {
		log.Println("addComment:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"post_id": id, "username": b.Username, "content": b.Content, "created_at": createdAt,
	})
}

// ---- 6. GET /posts/:id/comments ------------------------------------------------

func listComments(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if !postExists(id) {
		postNotFound(c)
		return
	}
	rows, err := db.Query(`
		SELECT post_id, username, content, created_at
		FROM comments WHERE post_id = ? ORDER BY id ASC`, id) // oldest first
	if err != nil {
		log.Println("listComments:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer rows.Close()

	list := []comment{}
	for rows.Next() {
		var cm comment
		if err := rows.Scan(&cm.PostID, &cm.Username, &cm.Content, &cm.CreatedAt); err != nil {
			log.Println("listComments scan:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		list = append(list, cm)
	}
	c.JSON(http.StatusOK, gin.H{"comments": list})
}

// ---- 7. GET /users/:username ----------------------------------------------------

func profile(c *gin.Context) {
	u := c.Param("username")
	var postCount int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts WHERE username = ?`, u).Scan(&postCount); err != nil {
		log.Println("profile:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if postCount == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Every successful request counts as one profile visit, saved (fsynced) first.
	if _, err := db.Exec(`
		INSERT INTO users (username, profile_visits) VALUES (?, 1)
		ON CONFLICT(username) DO UPDATE SET profile_visits = profile_visits + 1`, u); err != nil {
		log.Println("profile visit:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	var visits int64
	if err := db.QueryRow(`SELECT profile_visits FROM users WHERE username = ?`, u).Scan(&visits); err != nil {
		log.Println("profile read:", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"username": u, "post_count": postCount, "profile_visits": visits,
	})
}

// ---- server ----------------------------------------------------------------

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "challenge.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// rules.md: synchronous=FULL, every write fsynced before the reply.
	// WAL mode: completed commits survive kill -9 with no data loss.
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)"

	var err error
	if db, err = sql.Open("sqlite", dsn); err != nil {
		log.Fatal(err)
	}
	// Explicitly single connection: commits are serialized and deterministic.
	// Simple, predictable, reference-grade. Speed was not the goal.
	db.SetMaxOpenConns(1)

	if _, err = db.Exec(schema); err != nil {
		log.Fatal("schema:", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.HandleMethodNotAllowed = true

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})

	r.GET("/feed", feed)
	r.POST("/posts", createPost)
	r.GET("/posts/:id", getPost)
	r.POST("/posts/:id/like", likePost)
	r.POST("/posts/:id/comments", addComment)
	r.GET("/posts/:id/comments", listComments)
	r.GET("/users/:username", profile)

	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
