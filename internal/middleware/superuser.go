package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// IsSuperuserEmail checks if the email belongs to the superuser domain.
func IsSuperuserEmail(email, domain string) bool {
	if domain == "" || email == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(email), "@"+strings.ToLower(domain))
}

// RequireSuperuser returns middleware that checks if the current user is a superuser.
func RequireSuperuser(emailDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil || !IsSuperuserEmail(user.Email, emailDomain) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
