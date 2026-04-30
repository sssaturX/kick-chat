package licenseapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
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
		"license_key":     licenseKey,
		"expires_at":      expiresAt.UTC().Format(time.RFC3339),
		"max_activations": maxActivations,
	})
	status, respBody, err := c.postJSONWithRetry(ctx, c.baseURL+"/admin/licenses", body)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("create license: http %d: %s", status, string(respBody))
	}
	return nil
}

func (c *Client) AdminActivate(ctx context.Context, licenseKey string, expiresAt time.Time) error {
	body, _ := json.Marshal(map[string]string{
		"license_key": licenseKey,
		"expires_at":  expiresAt.UTC().Format(time.RFC3339),
	})
	status, respBody, err := c.postJSONWithRetry(ctx, c.baseURL+"/admin/activate", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("admin activate: http %d: %s", status, string(respBody))
	}
	return nil
}

func (c *Client) RevokeLicense(ctx context.Context, licenseKey string) error {
	body, _ := json.Marshal(map[string]string{"license_key": licenseKey})
	status, respBody, err := c.postJSONWithRetry(ctx, c.baseURL+"/admin/revoke", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("admin revoke: http %d: %s", status, string(respBody))
	}
	return nil
}

func (c *Client) ValidateLicense(ctx context.Context, licenseKey string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"license_key": licenseKey,
		"hwid":        "",
	})
	status, respBody, err := c.postJSONWithRetry(ctx, c.baseURL+"/validate", body)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("validate license: http %d: %s", status, string(respBody))
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if out.Status == "" {
		return "unknown", nil
	}
	return out.Status, nil
}

func (c *Client) postJSONWithRetry(ctx context.Context, url string, body []byte) (int, []byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Admin-API-Key", c.adminKey)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return resp.StatusCode, raw, nil
			}
			lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
		}
		time.Sleep(time.Duration(200*(attempt+1))*time.Millisecond + time.Duration(rand.Intn(120))*time.Millisecond)
	}
	return 0, nil, lastErr
}
