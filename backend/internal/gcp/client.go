package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Client struct {
	projectID string
	credsJSON []byte
}

func NewClient(creds models.GCPCredentials) *Client {
	return &Client{
		projectID: creds.ProjectID,
		credsJSON: []byte(creds.ServiceAccountJSON),
	}
}

func TestCredentials(ctx context.Context, creds models.GCPCredentials) error {
	client := NewClient(creds)
	if _, err := client.ListVMs(ctx); err != nil {
		return fmt.Errorf("failed to validate GCP credentials: %w", err)
	}
	return nil
}

func (c *Client) ListVMs(ctx context.Context) ([]models.VM, error) {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsJSON(c.credsJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP instances client: %w", err)
	}
	defer client.Close()

	req := &computepb.AggregatedListInstancesRequest{
		Project: c.projectID,
	}

	var vms []models.VM
	it := client.AggregatedList(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list GCP instances: %w", err)
		}

		zone := resp.Key
		for _, inst := range resp.Value.Instances {
			vms = append(vms, mapGCPInstance(inst, zone))
		}
	}

	return vms, nil
}

func (c *Client) StartVM(ctx context.Context, zone, name string) error {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsJSON(c.credsJSON))
	if err != nil {
		return fmt.Errorf("failed to create GCP instances client: %w", err)
	}
	defer client.Close()

	req := &computepb.StartInstanceRequest{
		Project:  c.projectID,
		Zone:     zone,
		Instance: name,
	}
	if _, err := client.Start(ctx, req); err != nil {
		return fmt.Errorf("failed to start GCP instance %s: %w", name, err)
	}
	return nil
}

func (c *Client) StopVM(ctx context.Context, zone, name string) error {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsJSON(c.credsJSON))
	if err != nil {
		return fmt.Errorf("failed to create GCP instances client: %w", err)
	}
	defer client.Close()

	req := &computepb.StopInstanceRequest{
		Project:  c.projectID,
		Zone:     zone,
		Instance: name,
	}
	if _, err := client.Stop(ctx, req); err != nil {
		return fmt.Errorf("failed to stop GCP instance %s: %w", name, err)
	}
	return nil
}

func mapGCPInstance(inst *computepb.Instance, zone string) models.VM {
	name := inst.GetName()
	vm := models.VM{
		ID:           fmt.Sprintf("%d", inst.GetId()),
		Provider:     models.ProviderGCP,
		Name:         name,
		State:        inst.GetStatus(),
		InstanceType: getMachineType(inst.GetMachineType()),
		Zone:         strings.TrimPrefix(zone, "zones/"),
		Tags:         inst.GetLabels(),
	}

	if len(inst.NetworkInterfaces) > 0 {
		ni := inst.NetworkInterfaces[0]
		vm.PrivateIP = ni.GetNetworkIP()
		if len(ni.AccessConfigs) > 0 {
			vm.PublicIP = ni.AccessConfigs[0].GetNatIP()
		}
	}

	if ts := inst.GetCreationTimestamp(); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			vm.LaunchTime = &t
		}
	}

	return vm
}

func getMachineType(machineType string) string {
	for i := len(machineType) - 1; i >= 0; i-- {
		if machineType[i] == '/' {
			return machineType[i+1:]
		}
	}
	return machineType
}
