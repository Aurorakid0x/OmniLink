package jwt

import (
	"OmniLink/pkg/back"
	"OmniLink/pkg/util/myjwt"
	"OmniLink/pkg/xerr"
	"strings"

	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			back.Error(c, xerr.Unauthorized, "missing or invalid authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := myjwt.ParseToken(tokenString)
		if err != nil {
			back.Error(c, xerr.Unauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Set("uuid", claims.Uuid)
		c.Set("username", claims.Username)
		c.Next()
	}
}
