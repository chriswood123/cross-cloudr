package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/chriswood123/cross-cloudr/backend/internal/auth"
	"github.com/chriswood123/cross-cloudr/backend/internal/aws"
	"github.com/chriswood123/cross-cloudr/backend/internal/gcp"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// Repository is the storage interface the handlers depend on.
// It is satisfied by *db.Repository.
type Repository interface {
	SaveCredentials(ctx context.Context, provider models.Provider, data interface{}) error
	GetCredentials(ctx context.Context, provider models.Provider, out interface{}) error
	DeleteCredentials(ctx context.Context, provider models.Provider) error
	ListCredentials(ctx context.Context) ([]models.Credentials, error)
}

type Handler struct {
	repo Repository
	auth *auth.Service

	// Injectable cloud-provider operations so tests can stub out real API calls.
	listAWSVMs   func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error)
	listGCPVMs   func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error)
	testAWSCreds func(ctx context.Context, creds models.AWSCredentials) error
	testGCPCreds func(ctx context.Context, creds models.GCPCredentials) error
}

func NewHandler(repo Repository, authService *auth.Service) *Handler {
	return &Handler{
		repo: repo,
		auth: authService,
		listAWSVMs: func(ctx context.Context, creds models.AWSCredentials) ([]models.VM, error) {
			client, err := aws.NewClient(ctx, creds)
			if err != nil {
				return nil, err
			}
			return client.ListVMs(ctx)
		},
		listGCPVMs: func(ctx context.Context, creds models.GCPCredentials) ([]models.VM, error) {
			client := gcp.NewClient(creds)
			return client.ListVMs(ctx)
		},
		testAWSCreds: aws.TestCredentials,
		testGCPCreds: gcp.TestCredentials,
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Username != h.auth.AdminUsername() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := h.auth.CheckPassword(req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.auth.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: token})
}

func (h *Handler) Me(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}

type saveAWSCredentialsRequest struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region"`
}

type saveGCPCredentialsRequest struct {
	ProjectID          string `json:"project_id"`
	ServiceAccountJSON string `json:"service_account_json"`
}

func validateProvider(c *gin.Context) (models.Provider, bool) {
	provider := models.Provider(c.Param("provider"))
	if provider != models.ProviderAWS && provider != models.ProviderGCP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
		return "", false
	}
	return provider, true
}

func (h *Handler) SaveCredentials(c *gin.Context) {
	provider, ok := validateProvider(c)
	if !ok {
		return
	}

	ctx := context.Background()
	if provider == models.ProviderAWS {
		var req saveAWSCredentialsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		existing := models.AWSCredentials{}
		hasExisting := h.repo.GetCredentials(ctx, provider, &existing) == nil

		creds := models.AWSCredentials{}
		if req.AccessKeyID != "" {
			creds.AccessKeyID = req.AccessKeyID
		} else if hasExisting {
			creds.AccessKeyID = existing.AccessKeyID
		}
		if req.SecretAccessKey != "" {
			creds.SecretAccessKey = req.SecretAccessKey
		} else if hasExisting {
			creds.SecretAccessKey = existing.SecretAccessKey
		}
		if req.Region != "" {
			creds.Region = req.Region
		} else if hasExisting {
			creds.Region = existing.Region
		}

		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" || creds.Region == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "access_key_id, secret_access_key, and region are required"})
			return
		}

		if err := h.repo.SaveCredentials(ctx, provider, creds); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		var req saveGCPCredentialsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		existing := models.GCPCredentials{}
		hasExisting := h.repo.GetCredentials(ctx, provider, &existing) == nil

		creds := models.GCPCredentials{}
		if req.ProjectID != "" {
			creds.ProjectID = req.ProjectID
		} else if hasExisting {
			creds.ProjectID = existing.ProjectID
		}
		if req.ServiceAccountJSON != "" {
			creds.ServiceAccountJSON = req.ServiceAccountJSON
		} else if hasExisting {
			creds.ServiceAccountJSON = existing.ServiceAccountJSON
		}

		if creds.ProjectID == "" || creds.ServiceAccountJSON == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id and service_account_json are required"})
			return
		}

		if err := h.repo.SaveCredentials(ctx, provider, creds); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "credentials saved"})
}

func (h *Handler) DeleteCredentials(c *gin.Context) {
	provider, ok := validateProvider(c)
	if !ok {
		return
	}

	if err := h.repo.DeleteCredentials(context.Background(), provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "credentials deleted"})
}

func (h *Handler) ListCredentials(c *gin.Context) {
	creds, err := h.repo.ListCredentials(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, creds)
}

type credentialDetail struct {
	Provider  models.Provider `json:"provider"`
	Region    string          `json:"region,omitempty"`
	ProjectID string          `json:"project_id,omitempty"`
	UpdatedAt string          `json:"updated_at"`
}

func (h *Handler) GetCredentials(c *gin.Context) {
	provider, ok := validateProvider(c)
	if !ok {
		return
	}

	ctx := context.Background()
	var updatedAt string
	if list, err := h.repo.ListCredentials(ctx); err == nil {
		for _, cred := range list {
			if cred.Provider == provider {
				updatedAt = cred.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
				break
			}
		}
	}

	if provider == models.ProviderAWS {
		var creds models.AWSCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, credentialDetail{
			Provider:  provider,
			Region:    creds.Region,
			UpdatedAt: updatedAt,
		})
	} else {
		var creds models.GCPCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, credentialDetail{
			Provider:  provider,
			ProjectID: creds.ProjectID,
			UpdatedAt: updatedAt,
		})
	}
}

func (h *Handler) TestCredentials(c *gin.Context) {
	provider, ok := validateProvider(c)
	if !ok {
		return
	}

	ctx := context.Background()
	if provider == models.ProviderAWS {
		var creds models.AWSCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.testAWSCreds(ctx, creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		var creds models.GCPCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.testGCPCreds(ctx, creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "credentials are valid"})
}

func (h *Handler) ListVMs(c *gin.Context) {
	ctx := context.Background()
	var allVMs []models.VM
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	var awsCreds models.AWSCredentials
	if err := h.repo.GetCredentials(ctx, models.ProviderAWS, &awsCreds); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vms, err := h.listAWSVMs(ctx, awsCreds)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("aws: %w", err))
				mu.Unlock()
				return
			}
			mu.Lock()
			allVMs = append(allVMs, vms...)
			mu.Unlock()
		}()
	}

	var gcpCreds models.GCPCredentials
	if err := h.repo.GetCredentials(ctx, models.ProviderGCP, &gcpCreds); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vms, err := h.listGCPVMs(ctx, gcpCreds)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("gcp: %w", err))
				mu.Unlock()
				return
			}
			mu.Lock()
			allVMs = append(allVMs, vms...)
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(allVMs) == 0 && len(errs) > 0 {
		errMsg := errs[0].Error()
		for _, e := range errs[1:] {
			errMsg += "; " + e.Error()
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	var warnings string
	if len(errs) > 0 {
		parts := make([]string, len(errs))
		for i, e := range errs {
			parts[i] = e.Error()
		}
		warnings = strings.Join(parts, "; ")
	}

	if allVMs == nil {
		allVMs = []models.VM{}
	}

	c.JSON(http.StatusOK, gin.H{"vms": allVMs, "warnings": warnings})
}
