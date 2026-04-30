package cryptopay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// Client talks to Telegram Crypto Pay API (@CryptoBot → Crypto Pay).
// Docs: https://help.crypt.bot/crypto-pay-api (see also https://help.send.tg for updates)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, apiToken string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   apiToken,
		http:    &http.Client{Timeout: 45 * time.Second},
	}
}

type apiResponse struct {
	Ok     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type Invoice struct {
	InvoiceID     int64  `json:"invoice_id"`
	Hash          string `json:"hash"`
	Asset         string `json:"asset"`
	Amount        string `json:"amount"`
	PayURL        string `json:"pay_url"`
	BotInvoiceURL string `json:"bot_invoice_url"`
	Status        string `json:"status"`
	Description   string `json:"description"`
	Payload       string `json:"payload"`
	CurrencyType  string `json:"currency_type"`
	Fiat          string `json:"fiat"`
	PaidAsset     string `json:"paid_asset"`
	PaidAmount    string `json:"paid_amount"`
	PaidFiat      string `json:"paid_fiat"`
	PaidFiatRate  string `json:"paid_fiat_rate"`
	PaidUsdRate   string `json:"paid_usd_rate"`
	PaidAt        string `json:"paid_at"`
}

func (c *Client) post(ctx context.Context, method string, params map[string]interface{}) (json.RawMessage, error) {
	if c.token == "" {
		return nil, fmt.Errorf("cryptopay: empty API token")
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	raw, status, err := c.doWithRetry(ctx, c.baseURL+"/"+method, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("cryptopay %s: http %d: %s", method, status, string(raw))
	}
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, fmt.Errorf("cryptopay: invalid json %s: %w", string(raw), err)
	}
	if !ar.Ok {
		return nil, fmt.Errorf("cryptopay %s: %s", method, string(ar.Error))
	}
	return ar.Result, nil
}

func (c *Client) doWithRetry(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Crypto-Pay-API-Token", c.token)
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return raw, resp.StatusCode, nil
			}
			lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
		}
		time.Sleep(time.Duration(200*(attempt+1))*time.Millisecond + time.Duration(rand.Intn(120))*time.Millisecond)
	}
	return nil, 0, lastErr
}

// CreateInvoiceUSDT creates an invoice for amount in USDT (string e.g. "29").
func (c *Client) CreateInvoiceUSDT(ctx context.Context, amountUSDT, description, payload string) (*Invoice, error) {
	params := map[string]interface{}{
		"asset":           "USDT",
		"amount":          amountUSDT,
		"description":     description,
		"payload":         payload,
		"allow_comments":  false,
		"allow_anonymous": false,
		"expires_in":      86400, // 24h to pay
	}
	raw, err := c.post(ctx, "createInvoice", params)
	if err != nil {
		return nil, err
	}
	var inv Invoice
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// GetInvoice returns one invoice by id (status: active | paid | expired).
func (c *Client) GetInvoice(ctx context.Context, invoiceID int64) (*Invoice, error) {
	tryParams := []map[string]interface{}{
		{"invoice_ids": fmt.Sprintf("%d", invoiceID)},
		{"invoice_ids": []int64{invoiceID}},
	}
	var lastErr error
	for _, params := range tryParams {
		raw, err := c.post(ctx, "getInvoices", params)
		if err != nil {
			lastErr = err
			continue
		}
		var wrap struct {
			Items []Invoice `json:"items"`
		}
		if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Items) > 0 {
			return &wrap.Items[0], nil
		}
		var arr []Invoice
		if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
			return &arr[0], nil
		}
		lastErr = fmt.Errorf("cryptopay: unexpected getInvoices shape: %s", string(raw))
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("cryptopay: invoice %d not found", invoiceID)
	}
	return nil, lastErr
}

// VerifyWebhookSignature checks Crypto Pay webhook (raw JSON body + secret token).
func VerifyWebhookSignature(body []byte, signatureHeader, apiToken string) bool {
	if signatureHeader == "" || apiToken == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(apiToken))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}
