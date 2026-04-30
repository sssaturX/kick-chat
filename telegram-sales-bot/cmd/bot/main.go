package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"kick-chat-go/telegram-sales-bot/internal/bot"
	"kick-chat-go/telegram-sales-bot/internal/config"
	"kick-chat-go/telegram-sales-bot/internal/cryptopay"
	"kick-chat-go/telegram-sales-bot/internal/licenseapi"
	"kick-chat-go/telegram-sales-bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	loadDotenv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	dbPath := os.Getenv("BOT_SQLITE_PATH")
	if dbPath == "" {
		dbPath = "telegram-sales-bot.db"
	}
	st, err := storage.Open(dbPath)
	if err != nil {
		log.Fatal("sqlite: ", err)
	}
	defer st.Close()

	lic := licenseapi.New(cfg.LicenseServerURL, cfg.AdminAPIKey)
	cp := cryptopay.New(cfg.CryptoPayBaseURL(), cfg.CryptoPayAPIToken)

	tg, err := tgbotapi.NewBotAPIWithClient(cfg.TelegramBotToken, tgbotapi.APIEndpoint, telegramHTTPClient())
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	tg.Debug = os.Getenv("TELEGRAM_DEBUG") == "1"
	log.Printf("Authorized as @%s", tg.Self.UserName)

	b := bot.New(cfg, tg, st, lic, cp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDailyReminders(ctx, cfg, b)
	go b.ResumeOpenInvoices(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tg.GetUpdatesChan(u)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		cancel()
		tg.StopReceivingUpdates()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown")
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			b.HandleUpdate(ctx, upd)
		}
	}
}

// loadDotenv loads .env from the process working directory, then from the directory
// containing the executable (so systemd and "binary + .env in same folder" work even if WorkingDirectory is wrong).
func loadDotenv() {
	_ = godotenv.Load()
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
		}
	}
}

// telegramHTTPClient is for long poll getUpdates(timeout=60); no HTTP(S)_PROXY / env proxy.
func telegramHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy:                 func(*http.Request) (*url.URL, error) { return nil, nil },
		TLSHandshakeTimeout:   25 * time.Second,
		ResponseHeaderTimeout: 0,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true,
		MaxIdleConnsPerHost:   -1,
	}
	return &http.Client{
		Timeout:   125 * time.Second,
		Transport: tr,
	}
}

func runDailyReminders(ctx context.Context, cfg *config.Config, b *bot.Bot) {
	hour := cfg.ReminderHourUTC
	if hour < 0 || hour > 23 {
		hour = 12
	}
	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		wait := time.Until(next)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			b.RunReminderScan(context.Background())
		}
	}
}
