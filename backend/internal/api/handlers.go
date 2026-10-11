package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/chriswood123/cross-cloudr/backend/internal/auth"
	"github.com/chriswood123/cross-cloudr/backend/internal/aws"
	"github.com/chriswood123/cross-cloudr/backend/internal/digitalocean"
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
	listDOVMs    func(ctx context.Context, creds models.DigitalOceanCredentials) ([]models.VM, error)
	testAWSCreds func(ctx context.Context, creds models.AWSCredentials) error
	testGCPCreds func(ctx context.Context, creds models.GCPCredentials) error
	testDOCreds  func(ctx context.Context, creds models.DigitalOceanCredentials) error
	startAWSVM   func(ctx context.Context, creds models.AWSCredentials, instanceID string) error
	stopAWSVM    func(ctx context.Context, creds models.AWSCredentials, instanceID string) error
	startGCPVM   func(ctx context.Context, creds models.GCPCredentials, zone, name string) error
	stopGCPVM    func(ctx context.Context, creds models.GCPCredentials, zone, name string) error
	startDOVM    func(ctx context.Context, creds models.DigitalOceanCredentials, id string) error
	stopDOVM     func(ctx context.Context, creds models.DigitalOceanCredentials, id string) error
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
		listDOVMs: func(ctx context.Context, creds models.DigitalOceanCredentials) ([]models.VM, error) {
			return digitalocean.NewClient(creds).ListVMs(ctx)
		},
		testAWSCreds: aws.TestCredentials,
		testGCPCreds: gcp.TestCredentials,
		testDOCreds:  digitalocean.TestCredentials,
		startAWSVM: func(ctx context.Context, creds models.AWSCredentials, instanceID string) error {
			client, err := aws.NewClient(ctx, creds)
			if err != nil {
				return err
			}
			return client.StartVM(ctx, instanceID)
		},
		stopAWSVM: func(ctx context.Context, creds models.AWSCredentials, instanceID string) error {
			client, err := aws.NewClient(ctx, creds)
			if err != nil {
				return err
			}
			return client.StopVM(ctx, instanceID)
		},
		startGCPVM: func(ctx context.Context, creds models.GCPCredentials, zone, name string) error {
			return gcp.NewClient(creds).StartVM(ctx, zone, name)
		},
		stopGCPVM: func(ctx context.Context, creds models.GCPCredentials, zone, name string) error {
			return gcp.NewClient(creds).StopVM(ctx, zone, name)
		},
		startDOVM: func(ctx context.Context, creds models.DigitalOceanCredentials, id string) error {
			return digitalocean.NewClient(creds).StartVM(ctx, id)
		},
		stopDOVM: func(ctx context.Context, creds models.DigitalOceanCredentials, id string) error {
			return digitalocean.NewClient(creds).StopVM(ctx, id)
		},
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

type saveDOCredentialsRequest struct {
	AccessToken string `json:"access_token"`
}

func validateProvider(c *gin.Context) (models.Provider, bool) {
	provider := models.Provider(c.Param("provider"))
	if provider != models.ProviderAWS && provider != models.ProviderGCP && provider != models.ProviderDO {
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
	} else if provider == models.ProviderGCP {
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
	} else {
		var req saveDOCredentialsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing := models.DigitalOceanCredentials{}
		hasExisting := h.repo.GetCredentials(ctx, provider, &existing) == nil
		creds := models.DigitalOceanCredentials{AccessToken: req.AccessToken}
		if creds.AccessToken == "" && hasExisting {
			creds.AccessToken = existing.AccessToken
		}
		if creds.AccessToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "access_token is required"})
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
	ctx := context.Background()
	creds, err := h.repo.ListCredentials(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]credentialDetail, 0, len(creds))
	for _, cred := range creds {
		detail := credentialDetail{
			Provider:  cred.Provider,
			UpdatedAt: cred.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		// Attach non-secret details so the UI can describe each credential.
		switch cred.Provider {
		case models.ProviderAWS:
			var awsCreds models.AWSCredentials
			if err := h.repo.GetCredentials(ctx, cred.Provider, &awsCreds); err == nil {
				detail.Region = awsCreds.Region
			}
		case models.ProviderGCP:
			var gcpCreds models.GCPCredentials
			if err := h.repo.GetCredentials(ctx, cred.Provider, &gcpCreds); err == nil {
				detail.ProjectID = gcpCreds.ProjectID
			}
		}
		result = append(result, detail)
	}

	c.JSON(http.StatusOK, result)
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
	} else if provider == models.ProviderGCP {
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
	} else {
		var creds models.DigitalOceanCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, credentialDetail{Provider: provider, UpdatedAt: updatedAt})
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
	} else if provider == models.ProviderGCP {
		var creds models.GCPCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.testGCPCreds(ctx, creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		var creds models.DigitalOceanCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err := h.testDOCreds(ctx, creds); err != nil {
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

	var doCreds models.DigitalOceanCredentials
	if err := h.repo.GetCredentials(ctx, models.ProviderDO, &doCreds); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vms, err := h.listDOVMs(ctx, doCreds)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("digitalocean: %w", err))
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

// vmActionRequest identifies a VM to start or stop. AWS and DigitalOcean
// address instances by ID; GCP addresses them by name within a zone.
type vmActionRequest struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	Zone       string `json:"zone"`
}

func (h *Handler) StartVM(c *gin.Context) {
	h.vmAction(c, "start")
}

func (h *Handler) StopVM(c *gin.Context) {
	h.vmAction(c, "stop")
}

func (h *Handler) vmAction(c *gin.Context, action string) {
	provider, ok := validateProvider(c)
	if !ok {
		return
	}

	var req vmActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if provider == models.ProviderAWS {
		if req.InstanceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "instance_id is required"})
			return
		}

		var creds models.AWSCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var err error
		if action == "start" {
			err = h.startAWSVM(ctx, creds, req.InstanceID)
		} else {
			err = h.stopAWSVM(ctx, creds, req.InstanceID)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if provider == models.ProviderGCP {
		if req.Name == "" || req.Zone == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and zone are required"})
			return
		}

		var creds models.GCPCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		zone := strings.TrimPrefix(req.Zone, "zones/")
		var err error
		if action == "start" {
			err = h.startGCPVM(ctx, creds, zone, req.Name)
		} else {
			err = h.stopGCPVM(ctx, creds, zone, req.Name)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if req.InstanceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "instance_id is required"})
			return
		}
		var creds models.DigitalOceanCredentials
		if err := h.repo.GetCredentials(ctx, provider, &creds); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		var err error
		if action == "start" {
			err = h.startDOVM(ctx, creds, req.InstanceID)
		} else {
			err = h.stopDOVM(ctx, creds, req.InstanceID)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("VM %s request accepted", action)})
}
