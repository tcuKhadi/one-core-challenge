package auth

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

func ValidUsername(u string) bool {
	return usernameRe.MatchString(u)
}

// Middleware verifies HS256 JWT tokens per api.md.
func Middleware(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}

		raw := strings.TrimSpace(h[len("Bearer "):])
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims := &jwt.RegisteredClaims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) {
			return secret, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

		if err != nil || !tok.Valid || !ValidUsername(claims.Subject) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("username", claims.Subject)
		c.Next()
	}
}
