package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chriswood123/cross-cloudr/backend/internal/encrypt"
	"github.com/chriswood123/cross-cloudr/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool      *pgxpool.Pool
	encryptor *encrypt.Encryptor
}

func NewRepository(pool *pgxpool.Pool, encryptor *encrypt.Encryptor) *Repository {
	return &Repository{pool: pool, encryptor: encryptor}
}

func (r *Repository) EnsureAdmin(ctx context.Context, username, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash
	`, username, passwordHash)
	return err
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, created_at FROM users WHERE username = $1
	`, username)

	user := &models.User{}
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *Repository) SaveCredentials(ctx context.Context, provider models.Provider, data interface{}) error {
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	encrypted, err := r.encryptor.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO cloud_credentials (provider, data)
		VALUES ($1, $2)
		ON CONFLICT (provider) DO UPDATE SET data = EXCLUDED.data, updated_at = NOW()
	`, provider, encrypted)
	return err
}

func (r *Repository) GetCredentials(ctx context.Context, provider models.Provider, out interface{}) error {
	row := r.pool.QueryRow(ctx, `
		SELECT data FROM cloud_credentials WHERE provider = $1
	`, provider)

	var encrypted []byte
	if err := row.Scan(&encrypted); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("no credentials found for provider %s", provider)
		}
		return err
	}

	plaintext, err := r.encryptor.Decrypt(encrypted)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	if err := json.Unmarshal(plaintext, out); err != nil {
		return fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return nil
}

func (r *Repository) DeleteCredentials(ctx context.Context, provider models.Provider) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cloud_credentials WHERE provider = $1`, provider)
	return err
}

func (r *Repository) ListCredentials(ctx context.Context) ([]models.Credentials, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, provider, created_at, updated_at FROM cloud_credentials`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.Credentials{}
	for rows.Next() {
		var cred models.Credentials
		if err := rows.Scan(&cred.ID, &cred.Provider, &cred.CreatedAt, &cred.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, cred)
	}
	return result, rows.Err()
}
