package digitalocean

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/chriswood123/cross-cloudr/backend/internal/models"
)

func TestListVMs(t *testing.T) {
	var requests []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing bearer token: %q", r.Header.Get("Authorization"))
		}
		requests = append(requests, r.URL.RequestURI())
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"droplets":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"droplets":[{"id":42,"name":"web-1","status":"active","size_slug":"s-1vcpu-1gb","region":{"slug":"nyc3"},"tags":["web","prod"],"created_at":"2025-01-02T03:04:05Z","networks":{"v4":[{"ip_address":"203.0.113.5","type":"public"},{"ip_address":"10.0.0.2","type":"private"}]}}],"links":{"pages":{"next":"` + server.URL + `/v2/droplets?page=2"}}}`))
	}))
	defer server.Close()
	client := NewClient(models.DigitalOceanCredentials{AccessToken: "test-token"})
	client.baseURL = server.URL + "/v2"
	client.httpClient = server.Client()

	vms, err := client.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("ListVMs returned an error: %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("expected one droplet, got %d", len(vms))
	}
	vm := vms[0]
	if vm.ID != "42" || vm.Provider != models.ProviderDO || vm.Name != "web-1" || vm.State != "running" {
		t.Errorf("unexpected droplet identity/state: %+v", vm)
	}
	if vm.InstanceType != "s-1vcpu-1gb" || vm.Region != "nyc3" {
		t.Errorf("unexpected size or region: %+v", vm)
	}
	if vm.PublicIP != "203.0.113.5" || vm.PrivateIP != "10.0.0.2" {
		t.Errorf("unexpected network addresses: %+v", vm)
	}
	if !reflect.DeepEqual(vm.Tags, map[string]string{"web": "", "prod": ""}) {
		t.Errorf("unexpected tags: %v", vm.Tags)
	}
	if vm.LaunchTime == nil || !vm.LaunchTime.Equal(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("unexpected launch time: %v", vm.LaunchTime)
	}
	if !reflect.DeepEqual(requests, []string{"/v2/droplets?per_page=200", "/v2/droplets?page=2"}) {
		t.Errorf("unexpected pagination requests: %v", requests)
	}
}

func TestPowerAction(t *testing.T) {
	for _, test := range []struct {
		name   string
		action func(*Client, context.Context, string) error
		want   string
	}{
		{name: "start", action: (*Client).StartVM, want: "power_on"},
		{name: "stop", action: (*Client).StopVM, want: "power_off"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v2/droplets/42/actions" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Errorf("missing token authorization")
				}
				var body struct {
					Type string `json:"type"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("failed to decode action body: %v", err)
				}
				if body.Type != test.want {
					t.Errorf("expected %q action, got %q", test.want, body.Type)
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()
			client := NewClient(models.DigitalOceanCredentials{AccessToken: "token"})
			client.baseURL = server.URL + "/v2"
			client.httpClient = server.Client()
			if err := test.action(client, context.Background(), "42"); err != nil {
				t.Fatalf("action returned an error: %v", err)
			}
		})
	}
}

func TestPowerActionRejectsInvalidID(t *testing.T) {
	client := NewClient(models.DigitalOceanCredentials{AccessToken: "token"})
	if err := client.StartVM(context.Background(), "not-a-number"); err == nil {
		t.Fatal("expected invalid droplet ID to be rejected")
	}
}
