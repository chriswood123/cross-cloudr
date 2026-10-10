package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	tokenExpiry           = 24 * time.Hour
)

type Service struct {
	secret        []byte
	adminUsername string
	adminHash     string
}

func NewService(secret, adminUsername, adminPassword string) *Service {
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	return &Service{
		secret:        []byte(secret),
		adminUsername: adminUsername,
		adminHash:     string(hash),
	}
}

func NewServiceWithHash(secret, adminUsername, adminHash string) *Service {
	return &Service{
		secret:        []byte(secret),
		adminUsername: adminUsername,
		adminHash:     adminHash,
	}
}

func (s *Service) AdminUsername() string {
	return s.adminUsername
}

func (s *Service) CheckPassword(password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(s.adminHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) GenerateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(tokenExpiry).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return s.secret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		sub, _ := claims["sub"].(string)
		if subtle.ConstantTimeCompare([]byte(sub), []byte(s.adminUsername)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token subject"})
			return
		}

		c.Set("username", sub)
		c.Next()
	}
}
