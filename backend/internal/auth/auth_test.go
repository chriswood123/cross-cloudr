package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const testPassword = "password123"

var testPasswordHash = func() string {
	h, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}()

func newTestService() *Service {
	return NewServiceWithHash("test-secret", "admin", testPasswordHash)
}

func TestAdminUsername(t *testing.T) {
	s := newTestService()
	if got := s.AdminUsername(); got != "admin" {
		t.Errorf("expected username admin, got %s", got)
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	s := newTestService()
	if err := s.CheckPassword(testPassword); err != nil {
		t.Errorf("expected correct password to pass, got error: %v", err)
	}
}

func TestCheckPassword_Incorrect(t *testing.T) {
	s := newTestService()
	if err := s.CheckPassword("wrong-password"); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestGenerateToken(t *testing.T) {
	s := newTestService()

	token, err := s.GenerateToken("admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	parsed, err := jwt.Parse(token, func(tok *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("generated token failed validation: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to extract claims")
	}
	if claims["sub"] != "admin" {
		t.Errorf("expected subject admin, got %v", claims["sub"])
	}

	// Expiry should be roughly 24 hours in the future.
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("failed to read expiry: %v", err)
	}
	remaining := time.Until(exp.Time)
	if remaining < 23*time.Hour || remaining > 25*time.Hour {
		t.Errorf("expected ~24h expiry, got %v", remaining)
	}
}

func newMiddlewareServer(s *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", s.Middleware(), func(c *gin.Context) {
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"username": username})
	})
	return r
}

func doAuthorizedRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMiddleware_ValidToken(t *testing.T) {
	s := newTestService()
	token, err := s.GenerateToken("admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	r := newMiddlewareServer(s)

	w := doAuthorizedRequest(r, "Bearer "+token)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"username":"admin"`) {
		t.Errorf("expected username in response, got %s", w.Body.String())
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	w := doAuthorizedRequest(r, "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMiddleware_MalformedHeader(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	for _, header := range []string{
		"Bearer",               // no token part
		"Basic abcdef",         // wrong scheme
		"Bearer   ",            // empty token
		"token-without-scheme", // no scheme at all
	} {
		w := doAuthorizedRequest(r, header)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for header %q, got %d", header, w.Code)
		}
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	w := doAuthorizedRequest(r, "Bearer not.a.real.token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMiddleware_WrongSigningSecret(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	// Sign a token with a different secret.
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("other-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	w := doAuthorizedRequest(r, "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for token signed with wrong secret, got %d", w.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	w := doAuthorizedRequest(r, "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestMiddleware_WrongSubject(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	claims := jwt.MapClaims{
		"sub": "attacker",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	w := doAuthorizedRequest(r, "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for token with wrong subject, got %d", w.Code)
	}
}

func TestMiddleware_NonHMACSigningMethod(t *testing.T) {
	s := newTestService()
	r := newMiddlewareServer(s)

	// Build an alg=none token; the middleware should reject the signing
	// method before even attempting verification.
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	w := doAuthorizedRequest(r, "Bearer "+token)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for alg=none token, got %d", w.Code)
	}
}
