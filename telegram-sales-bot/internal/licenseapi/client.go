package licenseapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	adminKey   string
	httpClient *http.Client
}

func New(baseURL, adminAPIKey string) *Client {
	return &Client{
		baseURL:  baseURL,
		adminKey: adminAPIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) CreateLicense(ctx context.Context, licenseKey string, expiresAt time.Time, maxActivations int) error {
	body, _ := json.Marshal(map[string]interface{}{
		"license_key":      licenseKey,
		"expires_at":       expiresAt.UTC().Format(time.RFC3339),
		"max_activations":  maxActivations,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/licenses", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-API-Key", c.adminKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create license: %s: %s", resp.Status, string(b))
	}
	return nil
}

func (c *Client) AdminActivate(ctx context.Context, licenseKey string, expiresAt time.Time) error {
	body, _ := json.Marshal(map[string]string{
		"license_key": licenseKey,
		"expires_at":  expiresAt.UTC().Format(time.RFC3339),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/activate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-API-Key", c.adminKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin activate: %s: %s", resp.Status, string(b))
	}
	return nil
}

func (c *Client) RevokeLicense(ctx context.Context, licenseKey string) error {
	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/admin/revoke", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-API-Key", c.adminKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin revoke: %s: %s", resp.Status, string(b))
	}
	return nil
}
