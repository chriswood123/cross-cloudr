package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/chriswood123/cross-cloudr/backend/internal/auth"
	"github.com/chriswood123/cross-cloudr/backend/internal/aws"
	"github.com/chriswood123/cross-cloudr/backend/internal/db"
	"github.com/chriswood123/cross-cloudr/backend/internal/gcp"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo   *db.Repository
	auth   *auth.Service
}

func NewHandler(repo *db.Repository, authService *auth.Service) *Handler {
	return &Handler{repo: repo, auth: authService}
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
	AccessKeyID     string `json:"access_key_id" binding:"required"`
	SecretAccessKey string `json:"secret_access_key" binding:"required"`
	Region          string `json:"region" binding:"required"`
}

type saveGCPCredentialsRequest struct {
	ProjectID          string `json:"project_id" binding:"required"`
	ServiceAccountJSON string `json:"service_account_json" binding:"required"`
}

func (h *Handler) SaveCredentials(c *gin.Context) {
	provider := models.Provider(c.Param("provider"))
	if provider != models.ProviderAWS && provider != models.ProviderGCP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
		return
	}

	ctx := context.Background()
	if provider == models.ProviderAWS {
		var req saveAWSCredentialsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		creds := models.AWSCredentials{
			AccessKeyID:     req.AccessKeyID,
			SecretAccessKey: req.SecretAccessKey,
			Region:          req.Region,
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
		creds := models.GCPCredentials{
			ProjectID:          req.ProjectID,
			ServiceAccountJSON: req.ServiceAccountJSON,
		}
		if err := h.repo.SaveCredentials(ctx, provider, creds); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "credentials saved"})
}

func (h *Handler) DeleteCredentials(c *gin.Context) {
	provider := models.Provider(c.Param("provider"))
	if provider != models.ProviderAWS && provider != models.ProviderGCP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
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
			client, err := aws.NewClient(ctx, awsCreds)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			vms, err := client.ListVMs(ctx)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
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
			client := gcp.NewClient(gcpCreds)
			vms, err := client.ListVMs(ctx)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": errs[0].Error()})
		return
	}

	c.JSON(http.StatusOK, allVMs)
}
