package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
)

type Client struct {
	ec2    *ec2.Client
	region string
}

func loadConfig(ctx context.Context, creds models.AWSCredentials) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			creds.SecretAccessKey,
			"",
		)),
	)
}

func NewClient(ctx context.Context, creds models.AWSCredentials) (*Client, error) {
	cfg, err := loadConfig(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{ec2: ec2.NewFromConfig(cfg), region: creds.Region}, nil
}

func TestCredentials(ctx context.Context, creds models.AWSCredentials) error {
	cfg, err := loadConfig(ctx, creds)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	if _, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
		return fmt.Errorf("failed to validate AWS credentials: %w", err)
	}
	return nil
}

func (c *Client) ListVMs(ctx context.Context) ([]models.VM, error) {
	input := &ec2.DescribeInstancesInput{}

	var vms []models.VM
	paginator := ec2.NewDescribeInstancesPaginator(c.ec2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe AWS instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				vm := mapEC2Instance(inst)
				vm.Region = c.region
				vms = append(vms, vm)
			}
		}
	}

	return vms, nil
}

func (c *Client) StartVM(ctx context.Context, instanceID string) error {
	input := &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	}
	if _, err := c.ec2.StartInstances(ctx, input); err != nil {
		return fmt.Errorf("failed to start AWS instance %s: %w", instanceID, err)
	}
	return nil
}

func (c *Client) StopVM(ctx context.Context, instanceID string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}
	if _, err := c.ec2.StopInstances(ctx, input); err != nil {
		return fmt.Errorf("failed to stop AWS instance %s: %w", instanceID, err)
	}
	return nil
}

func mapEC2Instance(inst types.Instance) models.VM {
	name := ""
	tags := make(map[string]string)
	for _, tag := range inst.Tags {
		if tag.Key != nil {
			tags[*tag.Key] = aws.ToString(tag.Value)
			if *tag.Key == "Name" {
				name = aws.ToString(tag.Value)
			}
		}
	}

	vm := models.VM{
		ID:           aws.ToString(inst.InstanceId),
		Provider:     models.ProviderAWS,
		Name:         name,
		State:        string(inst.State.Name),
		InstanceType: string(inst.InstanceType),
		Region:       "",
		Tags:         tags,
	}

	if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
		vm.Zone = *inst.Placement.AvailabilityZone
	}

	if ip := aws.ToString(inst.PublicIpAddress); ip != "" {
		vm.PublicIP = ip
	}
	if ip := aws.ToString(inst.PrivateIpAddress); ip != "" {
		vm.PrivateIP = ip
	}

	if inst.LaunchTime != nil {
		t := *inst.LaunchTime
		vm.LaunchTime = &t
	}

	return vm
}
