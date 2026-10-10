package models

import "time"

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Provider string

const (
	ProviderAWS Provider = "aws"
	ProviderGCP Provider = "gcp"
)

type Credentials struct {
	ID        int       `json:"id"`
	Provider  Provider  `json:"provider"`
	Data      []byte    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region"`
}

type GCPCredentials struct {
	ProjectID        string `json:"project_id"`
	ServiceAccountJSON string `json:"service_account_json"`
}

type VM struct {
	ID           string            `json:"id"`
	Provider     Provider          `json:"provider"`
	Name         string            `json:"name"`
	State        string            `json:"state"`
	InstanceType string            `json:"instance_type"`
	Region       string            `json:"region"`
	Zone         string            `json:"zone"`
	PublicIP     string            `json:"public_ip"`
	PrivateIP    string            `json:"private_ip"`
	LaunchTime   *time.Time        `json:"launch_time"`
	Tags         map[string]string `json:"tags"`
}
