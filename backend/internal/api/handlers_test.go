package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chriswood123/cross-cloudr/backend/internal/auth"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
	"github.com/gin-gonic/gin"
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

// mockRepo is an in-memory Repository for testing. It stores credentials as
// JSON, mirroring how the real repository marshals before encrypting.
type mockRepo struct {
	stored map[models.Provider][]byte
}

func newMockRepo() *mockRepo {
	return &mockRepo{stored: make(map[models.Provider][]byte)}
}

func (m *mockRepo) SaveCredentials(ctx context.Context, provider models.Provider, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m.stored[provider] = b
	return nil
}

func (m *mockRepo) GetCredentials(ctx context.Context, provider models.Provider, out interface{}) error {
	b, ok := m.stored[provider]
	if !ok {
		return fmt.Errorf("no credentials found for provider %s", provider)
	}
	return json.Unmarshal(b, out)
}

func (m *mockRepo) DeleteCredentials(ctx context.Context, provider models.Provider) error {
	delete(m.stored, provider)
	return nil
}

func (m *mockRepo) ListCredentials(ctx context.Context) ([]models.Credentials, error) {
	result := []models.Credentials{}
	now := time.Now()
	for provider := range m.stored {
		result = append(result, models.Credentials{
			ID:        1,
			Provider:  provider,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return result, nil
}

func newTestHandler(repo Repository) *Handler {
	authService := auth.NewServiceWithHash("test-secret", "admin", testPasswordHash)
	return NewHandler(repo, authService)
}

func newTestRouter(h *Handler, withAuthMiddleware bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Login is always public, mirroring the production routing.
	r.POST("/api/login", h.Login)

	var apiGroup *gin.RouterGroup
	if withAuthMiddleware {
		protected := r.Group("/api")
		protected.Use(h.auth.Middleware())
		apiGroup = protected
	} else {
		apiGroup = r.Group("/api")
	}
	apiGroup.GET("/me", h.Me)
	apiGroup.GET("/credentials", h.ListCredentials)
	apiGroup.GET("/credentials/:provider", h.GetCredentials)
	apiGroup.POST("/credentials/:provider", h.SaveCredentials)
	apiGroup.PUT("/credentials/:provider", h.SaveCredentials)
	apiGroup.DELETE("/credentials/:provider", h.DeleteCredentials)
	apiGroup.POST("/credentials/:provider/test", h.TestCredentials)
	apiGroup.GET("/vms", h.ListVMs)
	return r
}

func doRequest(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func loginToken(t *testing.T, r *gin.Engine) string {
	t.Helper()
	w := doRequest(r, "POST", "/api/login", `{"username":"admin","password":"`+testPassword+`"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	return resp.Token
}

func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, false)

	w := doRequest(r, "POST", "/api/login", `{"username":"admin","password":"`+testPassword+`"}`, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected token in response")
	}
}

func TestLogin_WrongUsername(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, false)

	w := doRequest(r, "POST", "/api/login", `{"username":"someone","password":"`+testPassword+`"}`, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, false)

	w := doRequest(r, "POST", "/api/login", `{"username":"admin","password":"wrong"}`, nil)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_MalformedBody(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, false)

	w := doRequest(r, "POST", "/api/login", `{"username":"admin"}`, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d", w.Code)
	}
}

// --- Auth middleware wiring ---

func TestProtectedEndpoints_RequireAuth(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/api/credentials"},
		{http.MethodGet, "/api/credentials/aws"},
		{http.MethodPost, "/api/credentials/aws"},
		{http.MethodDelete, "/api/credentials/aws"},
		{http.MethodPost, "/api/credentials/aws/test"},
		{http.MethodGet, "/api/vms"},
	}

	for _, ep := range endpoints {
		w := doRequest(r, ep.method, ep.path, "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s %s without token, got %d", ep.method, ep.path, w.Code)
		}
	}
}

func TestMe_WithValidToken(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)

	token := loginToken(t, r)
	w := doRequest(r, "GET", "/api/me", "", authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"username":"admin"`) {
		t.Errorf("expected username in response, got %s", w.Body.String())
	}
}

// --- Credentials ---

func TestSaveCredentials_AWS(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/aws",
		`{"access_key_id":"AKIATEST","secret_access_key":"secret","region":"us-east-1"}`,
		authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored models.AWSCredentials
	if err := repo.GetCredentials(context.Background(), models.ProviderAWS, &stored); err != nil {
		t.Fatalf("failed to read stored credentials: %v", err)
	}
	if stored.AccessKeyID != "AKIATEST" {
		t.Errorf("expected AKIATEST, got %s", stored.AccessKeyID)
	}
	if stored.SecretAccessKey != "secret" {
		t.Errorf("expected secret, got %s", stored.SecretAccessKey)
	}
	if stored.Region != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", stored.Region)
	}
}

func TestSaveCredentials_GCP(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/gcp",
		`{"project_id":"my-project","service_account_json":"{\"type\":\"service_account\"}"}`,
		authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored models.GCPCredentials
	if err := repo.GetCredentials(context.Background(), models.ProviderGCP, &stored); err != nil {
		t.Fatalf("failed to read stored credentials: %v", err)
	}
	if stored.ProjectID != "my-project" {
		t.Errorf("expected my-project, got %s", stored.ProjectID)
	}
	if stored.ServiceAccountJSON == "" {
		t.Error("expected service account JSON to be stored")
	}
}

func TestSaveCredentials_InvalidProvider(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/azure", `{}`, authHeaders(token))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown provider, got %d", w.Code)
	}
}

func TestSaveCredentials_AWS_PartialUpdate(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	// Save initial credentials.
	w := doRequest(r, "POST", "/api/credentials/aws",
		`{"access_key_id":"AKIAOLD","secret_access_key":"old-secret","region":"us-east-1"}`,
		authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("initial save failed: %d: %s", w.Code, w.Body.String())
	}

	// Update only the region; access key and secret must be preserved.
	w = doRequest(r, "PUT", "/api/credentials/aws", `{"region":"eu-west-1"}`, authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("partial update failed: %d: %s", w.Code, w.Body.String())
	}

	var stored models.AWSCredentials
	if err := repo.GetCredentials(context.Background(), models.ProviderAWS, &stored); err != nil {
		t.Fatalf("failed to read stored credentials: %v", err)
	}
	if stored.AccessKeyID != "AKIAOLD" {
		t.Errorf("expected access key to be preserved, got %s", stored.AccessKeyID)
	}
	if stored.SecretAccessKey != "old-secret" {
		t.Errorf("expected secret to be preserved, got %s", stored.SecretAccessKey)
	}
	if stored.Region != "eu-west-1" {
		t.Errorf("expected region updated to eu-west-1, got %s", stored.Region)
	}
}

func TestSaveCredentials_GCP_PartialUpdate(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/gcp",
		`{"project_id":"old-project","service_account_json":"old-json"}`,
		authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("initial save failed: %d: %s", w.Code, w.Body.String())
	}

	// Update only the project ID; the service account must be preserved.
	w = doRequest(r, "PUT", "/api/credentials/gcp", `{"project_id":"new-project"}`, authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("partial update failed: %d: %s", w.Code, w.Body.String())
	}

	var stored models.GCPCredentials
	if err := repo.GetCredentials(context.Background(), models.ProviderGCP, &stored); err != nil {
		t.Fatalf("failed to read stored credentials: %v", err)
	}
	if stored.ProjectID != "new-project" {
		t.Errorf("expected project updated to new-project, got %s", stored.ProjectID)
	}
	if stored.ServiceAccountJSON != "old-json" {
		t.Errorf("expected service account to be preserved, got %s", stored.ServiceAccountJSON)
	}
}

func TestSaveCredentials_MissingFieldsWithoutExisting(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/aws", `{"region":"us-east-1"}`, authHeaders(token))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when creating with missing fields, got %d", w.Code)
	}
}

func TestGetCredentials_AWS(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)

	w := doRequest(r, "GET", "/api/credentials/aws", "", authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var detail credentialDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if detail.Provider != models.ProviderAWS {
		t.Errorf("expected provider aws, got %s", detail.Provider)
	}
	if detail.Region != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", detail.Region)
	}

	// Secrets must never be exposed.
	if strings.Contains(w.Body.String(), "secret") {
		t.Errorf("response must not contain secrets: %s", w.Body.String())
	}
}

func TestGetCredentials_GCP(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedGCP(t, repo)

	w := doRequest(r, "GET", "/api/credentials/gcp", "", authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var detail credentialDetail
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if detail.ProjectID != "my-project" {
		t.Errorf("expected my-project, got %s", detail.ProjectID)
	}

	// Service account JSON must never be exposed.
	if strings.Contains(w.Body.String(), "service_account") {
		t.Errorf("response must not contain service account JSON: %s", w.Body.String())
	}
}

func TestGetCredentials_NotFound(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "GET", "/api/credentials/aws", "", authHeaders(token))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetCredentials_InvalidProvider(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "GET", "/api/credentials/azure", "", authHeaders(token))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteCredentials(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)

	w := doRequest(r, "DELETE", "/api/credentials/aws", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := repo.GetCredentials(context.Background(), models.ProviderAWS, &models.AWSCredentials{}); err == nil {
		t.Error("expected credentials to be deleted")
	}
}

func TestListCredentials(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)
	seedGCP(t, repo)

	w := doRequest(r, "GET", "/api/credentials", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var creds []models.Credentials
	if err := json.Unmarshal(w.Body.Bytes(), &creds); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(creds) != 2 {
		t.Errorf("expected 2 credential records, got %d", len(creds))
	}
}

func TestListCredentials_EmptyReturnsArray(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "GET", "/api/credentials", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// The API must return an empty array, not null.
	if strings.TrimSpace(w.Body.String()) == "null" {
		t.Error("expected empty array, got null")
	}
	var creds []models.Credentials
	if err := json.Unmarshal(w.Body.Bytes(), &creds); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected 0 credential records, got %d", len(creds))
	}
}

// --- Credential testing ---

func TestTestCredentials_AWS_Success(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.testAWSCreds = func(ctx context.Context, creds models.AWSCredentials) error {
		return nil
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)

	w := doRequest(r, "POST", "/api/credentials/aws/test", "", authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestCredentials_AWS_Failure(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.testAWSCreds = func(ctx context.Context, creds models.AWSCredentials) error {
		return fmt.Errorf("InvalidClientTokenId: The security token included in the request is invalid")
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)

	w := doRequest(r, "POST", "/api/credentials/aws/test", "", authHeaders(token))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "InvalidClientTokenId") {
		t.Errorf("expected error detail in response, got %s", w.Body.String())
	}
}

func TestTestCredentials_GCP_Success(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.testGCPCreds = func(ctx context.Context, creds models.GCPCredentials) error {
		return nil
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedGCP(t, repo)

	w := doRequest(r, "POST", "/api/credentials/gcp/test", "", authHeaders(token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTestCredentials_MissingCredentials(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "POST", "/api/credentials/aws/test", "", authHeaders(token))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- VM listing ---

func TestListVMs_BothProviders(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.listAWSVMs = func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error) {
		return []models.VM{{ID: "i-123", Provider: models.ProviderAWS, Name: "web-1", State: "running"}}, nil
	}
	h.listGCPVMs = func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error) {
		return []models.VM{{ID: "456", Provider: models.ProviderGCP, Name: "gce-1", State: "RUNNING"}}, nil
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)
	seedGCP(t, repo)

	w := doRequest(r, "GET", "/api/vms", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		VMs      []models.VM `json:"vms"`
		Warnings string      `json:"warnings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.VMs) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(resp.VMs))
	}
	if resp.Warnings != "" {
		t.Errorf("expected no warnings, got %q", resp.Warnings)
	}
}

func TestListVMs_PartialFailure(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.listAWSVMs = func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error) {
		return nil, fmt.Errorf("failed to describe AWS instances")
	}
	h.listGCPVMs = func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error) {
		return []models.VM{{ID: "456", Provider: models.ProviderGCP, Name: "gce-1", State: "RUNNING"}}, nil
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)
	seedGCP(t, repo)

	w := doRequest(r, "GET", "/api/vms", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		VMs      []models.VM `json:"vms"`
		Warnings string      `json:"warnings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.VMs) != 1 {
		t.Errorf("expected GCP VMs despite AWS failure, got %d", len(resp.VMs))
	}
	if !strings.Contains(resp.Warnings, "aws:") {
		t.Errorf("expected AWS warning, got %q", resp.Warnings)
	}
}

func TestListVMs_AllProvidersFail(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.listAWSVMs = func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error) {
		return nil, fmt.Errorf("aws down")
	}
	h.listGCPVMs = func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error) {
		return nil, fmt.Errorf("gcp down")
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)
	seedGCP(t, repo)

	w := doRequest(r, "GET", "/api/vms", "", authHeaders(token))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when all providers fail, got %d", w.Code)
	}
}

func TestListVMs_NoCredentials(t *testing.T) {
	h := newTestHandler(newMockRepo())
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	w := doRequest(r, "GET", "/api/vms", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with no credentials configured, got %d: %s", w.Code, w.Body.String())
	}

	// Must be an empty array, not null.
	if !strings.Contains(w.Body.String(), `"vms":[]`) {
		t.Errorf("expected empty vms array, got %s", w.Body.String())
	}
}

func TestListVMs_OnlyAWSConfigured(t *testing.T) {
	repo := newMockRepo()
	h := newTestHandler(repo)
	h.listAWSVMs = func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error) {
		return []models.VM{{ID: "i-123", Provider: models.ProviderAWS, Name: "web-1"}}, nil
	}
	h.listGCPVMs = func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error) {
		t.Error("GCP list must not be called when GCP credentials are absent")
		return nil, nil
	}
	r := newTestRouter(h, true)
	token := loginToken(t, r)

	seedAWS(t, repo)

	w := doRequest(r, "GET", "/api/vms", "", authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		VMs []models.VM `json:"vms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.VMs) != 1 {
		t.Errorf("expected 1 VM, got %d", len(resp.VMs))
	}
}

// --- Helpers ---

func seedAWS(t *testing.T, repo *mockRepo) {
	t.Helper()
	err := repo.SaveCredentials(context.Background(), models.ProviderAWS, models.AWSCredentials{
		AccessKeyID:     "AKIATEST",
		SecretAccessKey: "secret",
		Region:          "us-east-1",
	})
	if err != nil {
		t.Fatalf("failed to seed AWS credentials: %v", err)
	}
}

func seedGCP(t *testing.T, repo *mockRepo) {
	t.Helper()
	err := repo.SaveCredentials(context.Background(), models.ProviderGCP, models.GCPCredentials{
		ProjectID:          "my-project",
		ServiceAccountJSON: `{"type":"service_account"}`,
	})
	if err != nil {
		t.Fatalf("failed to seed GCP credentials: %v", err)
	}
}
