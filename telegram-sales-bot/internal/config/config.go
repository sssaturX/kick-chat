package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken string

	// Crypto Pay (@CryptoBot → Crypto Pay → Create App)
	CryptoPayAPIToken string
	CryptoPayTestnet  bool

	// License server (same as SaturX build)
	LicenseServerURL string
	AdminAPIKey      string

	// Pricing in USDT (Crypto Pay bills in crypto; amounts are approximate to USD)
	PriceStandardUSDT string // e.g. "29"
	PriceProUSDT      string // e.g. "129"

	// Subscription length (Standard)
	PeriodDays int
	// Pro plan length (default 365 = 1 year)
	PeriodDaysPro int

	// New license: max device activations on license server
	MaxActivations int
	// Trial license settings
	TrialPeriodHours    int
	TrialMaxActivations int

	// Optional: image URL shown on /start (Telegram will fetch it)
	WelcomePhotoURL string
	// Optional: local path to PNG/JPEG for /start (no public URL). Tried before default filenames.
	WelcomePhotoPath string

	// Poll invoice every N seconds until paid
	InvoicePollSeconds    int
	InvoicePollTimeoutMin int

	// Reminder job
	ReminderHourUTC int // 0-23, default 12
	// Referral bonus applied to inviter after a referred buyer is fulfilled.
	ReferralBonusDays int

	// Telegram user IDs allowed to use /admin (comma-separated), e.g. 123456789,987654321
	TelegramAdminIDs map[int64]struct{}

	// Optional: HTTPS link to SaturX zip/installer (shown after payment and in /start)
	SoftwareDownloadURL      string
	SoftwareDownloadLinkText string // button/caption text, default "Download SaturX"
}

func Load() (*Config, error) {
	c := &Config{
		PriceStandardUSDT:   getenv("PRICE_STANDARD_USDT", "29"),
		PriceProUSDT:        getenv("PRICE_PRO_USDT", "129"),
		PeriodDays:          getenvInt("SUBSCRIPTION_PERIOD_DAYS", 30),
		PeriodDaysPro:       getenvInt("SUBSCRIPTION_PERIOD_DAYS_PRO", 365),
		MaxActivations:      getenvInt("LICENSE_MAX_ACTIVATIONS", 2),
		TrialPeriodHours:    getenvInt("TRIAL_PERIOD_HOURS", 72),
		TrialMaxActivations: getenvInt("TRIAL_MAX_ACTIVATIONS", 1),
		WelcomePhotoURL:     strings.TrimSpace(os.Getenv("WELCOME_PHOTO_URL")),
		WelcomePhotoPath:    strings.TrimSpace(os.Getenv("WELCOME_PHOTO_PATH")),
		InvoicePollSeconds:  getenvInt("INVOICE_POLL_SECONDS", 12),
		// Default 1440 min = 24h, aligned with Crypto Pay invoice expires_in in cryptopay client
		InvoicePollTimeoutMin: getenvInt("INVOICE_POLL_TIMEOUT_MIN", 1440),
		ReminderHourUTC:       getenvInt("REMINDER_HOUR_UTC", 12),
		ReferralBonusDays:     getenvInt("REFERRAL_BONUS_DAYS", 7),
	}
	c.TelegramBotToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if c.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	c.CryptoPayAPIToken = strings.TrimSpace(os.Getenv("CRYPTOPAY_API_TOKEN"))
	c.CryptoPayTestnet = strings.EqualFold(strings.TrimSpace(os.Getenv("CRYPTOPAY_TESTNET")), "true") ||
		os.Getenv("CRYPTOPAY_TESTNET") == "1"
	c.LicenseServerURL = strings.TrimSuffix(strings.TrimSpace(os.Getenv("LICENSE_SERVER_URL")), "/")
	c.AdminAPIKey = strings.TrimSpace(os.Getenv("LICENSE_ADMIN_API_KEY"))
	if c.LicenseServerURL == "" || c.AdminAPIKey == "" {
		return nil, fmt.Errorf("LICENSE_SERVER_URL and LICENSE_ADMIN_API_KEY are required")
	}
	c.TelegramAdminIDs = parseAdminIDs(os.Getenv("TELEGRAM_ADMIN_IDS"))

	c.SoftwareDownloadURL = strings.TrimSpace(os.Getenv("SOFTWARE_DOWNLOAD_URL"))
	c.SoftwareDownloadLinkText = strings.TrimSpace(os.Getenv("SOFTWARE_DOWNLOAD_LINK_TEXT"))
	if c.SoftwareDownloadURL != "" {
		u, perr := url.Parse(c.SoftwareDownloadURL)
		if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("SOFTWARE_DOWNLOAD_URL must be a full http(s) URL to your zip or installer")
		}
	}

	return c, nil
}

func parseAdminIDs(s string) map[int64]struct{} {
	out := make(map[int64]struct{})
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func (c *Config) IsAdmin(telegramUserID int64) bool {
	if len(c.TelegramAdminIDs) == 0 {
		return false
	}
	_, ok := c.TelegramAdminIDs[telegramUserID]
	return ok
}

func (c *Config) CryptoPayBaseURL() string {
	if c.CryptoPayTestnet {
		return "https://testnet-pay.crypt.bot/api"
	}
	return "https://pay.crypt.bot/api"
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
