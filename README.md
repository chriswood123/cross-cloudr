# Cross Cloudr

A simple web dashboard for viewing cloud compute resources across AWS and Google Cloud.

## Features (MVP)

- View-only VM dashboard for AWS EC2 and GCP Compute Engine
- Encrypted cloud credential storage
- Single admin user authentication
- Docker Compose for local development
- Helm chart for Kubernetes deployment

## Tech Stack

- **Backend:** Go + Gin
- **Frontend:** Vue 3 + Vite + CoreUI
- **Database:** PostgreSQL

## Local Development

1. Copy the example environment file and configure it:

   ```bash
   cp .env.example .env
   ```

   Generate a secure encryption key:

   ```bash
   openssl rand -base64 32
   ```

2. Start the stack with Docker Compose:

   ```bash
   docker compose up --build
   ```

3. Open the frontend at http://localhost:3000 and log in with the admin credentials from `.env`.

## Adding Cloud Credentials

1. Log in to the dashboard.
2. Navigate to **Credentials**.
3. Add your AWS IAM user access keys and region, or upload a GCP service account JSON.
4. Go to **VMs** to view your instances.

## Kubernetes Deployment

Build and tag the images:

```bash
docker build -t cross-cloudr-backend:latest ./backend
docker build -t cross-cloudr-frontend:latest ./frontend
```

Generate secrets and install the Helm chart:

```bash
helm dependency build ./helm/cross-cloudr
helm install cross-cloudr ./helm/cross-cloudr \
  --set admin.encryptionKey=$(openssl rand -base64 32) \
  --set admin.sessionSecret=$(openssl rand -base64 32) \
  --set admin.password=changeme
```

## Running Tests

Backend (requires Go 1.26+):

```bash
cd backend
go test ./...
```

Frontend (requires Node 20+):

```bash
cd frontend
npm install
npm test
```

## Security Notes

- Cloud credentials are encrypted at rest with AES-256-GCM.
- Change the default admin password and use strong secrets in production.
- Run behind HTTPS when deployed.
