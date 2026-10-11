package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chriswood123/cross-cloudr/backend/internal/models"
)

const apiURL = "https://api.digitalocean.com/v2"

type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

func NewClient(creds models.DigitalOceanCredentials) *Client {
	return &Client{
		token:      creds.AccessToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    apiURL,
	}
}

func TestCredentials(ctx context.Context, creds models.DigitalOceanCredentials) error {
	client := NewClient(creds)
	var account json.RawMessage
	if err := client.get(ctx, "/account", &account); err != nil {
		return fmt.Errorf("failed to validate DigitalOcean credentials: %w", err)
	}
	return nil
}

type digitalDroplet struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	SizeSlug string `json:"size_slug"`
	Region   struct {
		Slug string `json:"slug"`
	} `json:"region"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	Networks  struct {
		V4 []struct {
			IPAddress string `json:"ip_address"`
			Type      string `json:"type"`
		} `json:"v4"`
	} `json:"networks"`
}

type dropletListResponse struct {
	Droplets []digitalDroplet `json:"droplets"`
	Links    struct {
		Pages struct {
			Next string `json:"next"`
		} `json:"pages"`
	} `json:"links"`
}

func (c *Client) ListVMs(ctx context.Context) ([]models.VM, error) {
	var vms []models.VM
	path := "/droplets?per_page=200"
	for path != "" {
		var response dropletListResponse
		if err := c.get(ctx, path, &response); err != nil {
			return nil, fmt.Errorf("failed to list DigitalOcean droplets: %w", err)
		}
		for _, droplet := range response.Droplets {
			vms = append(vms, mapDroplet(droplet))
		}
		path = ""
		if response.Links.Pages.Next != "" {
			// DigitalOcean supplies an absolute URL for the next page.
			next, err := url.Parse(response.Links.Pages.Next)
			if err != nil {
				return nil, fmt.Errorf("invalid DigitalOcean pagination URL: %w", err)
			}
			base, err := url.Parse(c.baseURL)
			if err != nil {
				return nil, fmt.Errorf("invalid DigitalOcean API URL: %w", err)
			}
			if next.Host != base.Host || !strings.HasPrefix(next.Path, strings.TrimSuffix(base.Path, "/")+"/") {
				return nil, fmt.Errorf("unexpected DigitalOcean pagination URL: %s", next)
			}
			next.Path = strings.TrimPrefix(next.Path, strings.TrimSuffix(base.Path, "/"))
			path = next.RequestURI()
		}
	}
	return vms, nil
}

func (c *Client) StartVM(ctx context.Context, id string) error {
	return c.powerAction(ctx, id, "power_on")
}

func (c *Client) StopVM(ctx context.Context, id string) error {
	return c.powerAction(ctx, id, "power_off")
}

func (c *Client) powerAction(ctx context.Context, id, action string) error {
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return fmt.Errorf("invalid DigitalOcean droplet ID %q", id)
	}
	body := strings.NewReader(`{"type":"` + action + `"}`)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/droplets/"+url.PathEscape(id)+"/actions", body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return c.do(request, nil)
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(request, out)
}

func (c *Client) do(request *http.Request, out interface{}) error {
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var apiError struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(response.Body).Decode(&apiError)
		if apiError.Message == "" {
			apiError.Message = response.Status
		}
		return fmt.Errorf("DigitalOcean API returned %s: %s", response.Status, apiError.Message)
	}
	if out != nil {
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			return fmt.Errorf("failed to decode DigitalOcean API response: %w", err)
		}
	}
	return nil
}

func mapDroplet(d digitalDroplet) models.VM {
	state := d.Status
	if state == "active" {
		state = "running"
	} else if state == "off" {
		state = "stopped"
	}
	// DigitalOcean tags are labels without values; an empty string marks that.
	tags := make(map[string]string, len(d.Tags))
	for _, tag := range d.Tags {
		tags[tag] = ""
	}
	vm := models.VM{
		ID:           strconv.Itoa(d.ID),
		Provider:     models.ProviderDO,
		Name:         d.Name,
		State:        state,
		InstanceType: d.SizeSlug,
		Region:       d.Region.Slug,
		Tags:         tags,
	}
	for _, network := range d.Networks.V4 {
		if network.Type == "public" {
			vm.PublicIP = network.IPAddress
		} else if network.Type == "private" {
			vm.PrivateIP = network.IPAddress
		}
	}
	if createdAt, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
		vm.LaunchTime = &createdAt
	}
	return vm
}
