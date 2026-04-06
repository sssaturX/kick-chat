package bot

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kick-chat-go/telegram-sales-bot/internal/config"
	"kick-chat-go/telegram-sales-bot/internal/cryptopay"
	"kick-chat-go/telegram-sales-bot/internal/licenseapi"
	"kick-chat-go/telegram-sales-bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	tierStandard = "standard"
	tierPro      = "pro"
)

type Bot struct {
	cfg    *config.Config
	api    *tgbotapi.BotAPI
	store  *storage.Storage
	lic    *licenseapi.Client
	cp     *cryptopay.Client
	log    *log.Logger
}

func New(cfg *config.Config, api *tgbotapi.BotAPI, st *storage.Storage, lic *licenseapi.Client, cp *cryptopay.Client) *Bot {
	return &Bot{cfg: cfg, api: api, store: st, lic: lic, cp: cp, log: log.Default()}
}

func GenLicenseKey() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	part := func(n int) string {
		buf := make([]byte, n)
		_, _ = rand.Read(buf)
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[int(buf[i])%len(alphabet)]
		}
		return string(b)
	}
	return part(4) + "-" + part(4) + "-" + part(4) + "-" + part(4)
}

func (b *Bot) welcomeCaption() string {
	s := `🟢 <b>SaturX</b> — Kick chat dashboard in one place.

<b>You get:</b>
• Embedded channel chat + live stream
• Multiple Kick accounts, switch in one click
• Message presets (<code>messages.txt</code>) + auto-send
• Per-account SOCKS5 proxy in the dashboard
• Optional viewer boost tab (same app)
• License tied to your purchase — keys issued automatically after payment

<b>Plans</b>
• <b>Standard — $29/mo</b> (billed in USDT via Crypto Pay)
• <b>Pro — $129/yr</b> — everything in Standard + <i>AI bots in advance in the next update</i>

Tap a button below to pay. After payment you receive a license key for SaturX and renewal reminders here.

Questions? <a href="https://t.me/ssaturx">@ssaturx</a> or /support.`
	return s
}

// softwareDownloadKeyboard is an inline URL button after payment / My license (not shown on /start).
func (b *Bot) softwareDownloadKeyboard() *tgbotapi.InlineKeyboardMarkup {
	u := strings.TrimSpace(b.cfg.SoftwareDownloadURL)
	if u == "" {
		return nil
	}
	label := strings.TrimSpace(b.cfg.SoftwareDownloadLinkText)
	if label == "" {
		label = "Download SaturX"
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📦 "+label, u),
		),
	)
	return &kb
}

func (b *Bot) mainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Std — $29/mo", "buy:"+tierStandard),
			tgbotapi.NewInlineKeyboardButtonData("⭐ Pro — $129/yr", "buy:"+tierPro),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔑 My license", "mylicense"),
		),
	)
}

func (b *Bot) HandleUpdate(ctx context.Context, u tgbotapi.Update) {
	switch {
	case u.Message != nil && u.Message.IsCommand():
		b.handleCommand(ctx, u.Message)
	case u.CallbackQuery != nil:
		b.handleCallback(ctx, u.CallbackQuery)
	}
}

func (b *Bot) handleCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		b.sendWelcome(m.Chat.ID, nil)
	case "support":
		b.sendHTML(m.Chat.ID, `📩 <b>Support</b>

Manager: <a href="https://t.me/ssaturx">@ssaturx</a>

Write in Telegram for payment issues, license help, or product questions.`)
	case "admin", "ahelp":
		b.handleAdmin(ctx, m)
	default:
		msg := tgbotapi.NewMessage(m.Chat.ID, "Use /start for plans. /support for the manager.")
		_, _ = b.api.Send(msg)
	}
}

func (b *Bot) handleAdmin(ctx context.Context, m *tgbotapi.Message) {
	if !b.cfg.IsAdmin(m.From.ID) {
		msg := tgbotapi.NewMessage(m.Chat.ID, "Use /start for plans. /support for the manager.")
		_, _ = b.api.Send(msg)
		return
	}
	cmd := m.Command()
	args := strings.Fields(strings.TrimSpace(m.CommandArguments()))
	if cmd == "ahelp" || (cmd == "admin" && len(args) == 0) {
		b.sendAdminHelp(m.Chat.ID)
		return
	}
	if cmd != "admin" {
		b.sendAdminHelp(m.Chat.ID)
		return
	}
	switch args[0] {
	case "stats":
		nSub, err1 := b.store.CountSubscriptions(ctx)
		nPen, err2 := b.store.CountPendingInvoices(ctx)
		if err1 != nil || err2 != nil {
			b.sendText(m.Chat.ID, fmt.Sprintf("DB error: sub %v, pending %v", err1, err2))
			return
		}
		b.sendText(m.Chat.ID, fmt.Sprintf("Subscriptions: %d\nPending Crypto Pay invoices: %d", nSub, nPen))
	case "user":
		if len(args) < 2 {
			b.sendText(m.Chat.ID, "Usage: /admin user <telegram_user_id>")
			return
		}
		uid, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			b.sendText(m.Chat.ID, "Invalid user id.")
			return
		}
		sub, err := b.store.GetSubscription(ctx, uid)
		if err != nil {
			b.sendText(m.Chat.ID, "DB error: "+err.Error())
			return
		}
		if sub == nil {
			b.sendText(m.Chat.ID, "No subscription for this Telegram id.")
			return
		}
		days := storage.FormatDaysLeft(sub.ExpiresAt)
		un := sub.Username
		if un != "" {
			un = "@" + un
		} else {
			un = "(no username)"
		}
		b.sendHTML(m.Chat.ID, fmt.Sprintf(
			"<b>User</b> <code>%d</code> %s\n<b>Key</b> <code>%s</code>\n<b>Tier</b> %s\n<b>Expires</b> %s UTC\n<b>Days left</b> %d",
			sub.TelegramUserID, un, sub.LicenseKey, sub.Tier, sub.ExpiresAt.UTC().Format(time.RFC3339), days,
		))
	case "pending":
		rows, err := b.store.ListPendingInvoices(ctx, 15)
		if err != nil {
			b.sendText(m.Chat.ID, "DB error: "+err.Error())
			return
		}
		if len(rows) == 0 {
			b.sendText(m.Chat.ID, "No pending invoices.")
			return
		}
		var sb strings.Builder
		sb.WriteString("<b>Pending invoices</b>\n")
		for _, r := range rows {
			sb.WriteString(fmt.Sprintf("inv <code>%d</code> tg <code>%d</code> %s\n", r.InvoiceID, r.TelegramUserID, r.Tier))
		}
		_ = b.sendHTML(m.Chat.ID, sb.String())
	case "droplocal":
		if len(args) < 2 {
			b.sendText(m.Chat.ID, "Usage: /admin droplocal <telegram_user_id>\nRemoves bot DB only; revoke on license server separately if needed.")
			return
		}
		uid, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			b.sendText(m.Chat.ID, "Invalid user id.")
			return
		}
		if err := b.store.DeleteSubscription(ctx, uid); err != nil {
			b.sendText(m.Chat.ID, "DB error: "+err.Error())
			return
		}
		b.sendText(m.Chat.ID, fmt.Sprintf("Removed bot records for tg %d (license server not changed).", uid))
	case "create":
		if len(args) < 3 {
			b.sendText(m.Chat.ID, "Usage: /admin create <telegram_user_id> <standard|pro>\nCreates license on license server + bot DB. Fails if this user already has a bot row (use droplocal + revoke first).")
			return
		}
		uid, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			b.sendText(m.Chat.ID, "Invalid telegram_user_id.")
			return
		}
		tier, ok := parseTierArg(args[2])
		if !ok {
			b.sendText(m.Chat.ID, "Tier must be standard or pro.")
			return
		}
		sub, err := b.store.GetSubscription(ctx, uid)
		if err != nil {
			b.sendText(m.Chat.ID, "DB error: "+err.Error())
			return
		}
		if sub != nil {
			b.sendText(m.Chat.ID, fmt.Sprintf("User %d already has a bot subscription. Use /admin droplocal %d and revoke the old key on the server if you need a new one.", uid, uid))
			return
		}
		key := GenLicenseKey()
		exp := time.Now().UTC().Add(b.subscriptionPeriodForTier(tier))
		if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil {
			b.sendText(m.Chat.ID, "License server error: "+err.Error())
			return
		}
		if err := b.store.InsertSubscription(ctx, uid, "", key, tier, exp); err != nil {
			b.sendText(m.Chat.ID, "Created on server but bot DB failed: "+err.Error()+"\nKey: "+key)
			return
		}
		userHTML := fmt.Sprintf(
			"🔑 <b>Your SaturX license</b>\n\nKey:\n<code>%s</code>\n\nExpires (UTC): %s\nActivations: up to %d devices.\n\nOpen SaturX and enter the key when asked.",
			key, exp.Format(time.RFC3339), b.cfg.MaxActivations,
		)
		if err := b.sendHTMLWithMarkup(uid, userHTML, b.softwareDownloadKeyboard()); err != nil {
			b.log.Printf("admin create: DM user %d: %v", uid, err)
			b.sendHTML(m.Chat.ID, fmt.Sprintf(
				"<b>Created</b> for <code>%d</code> (could not DM — user may need to /start the bot first).\n\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s",
				uid, key, tier, exp.Format(time.RFC3339),
			))
			return
		}
		b.sendHTML(m.Chat.ID, fmt.Sprintf(
			"<b>Created</b> for <code>%d</code> — DM sent.\n\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s",
			uid, key, tier, exp.Format(time.RFC3339),
		))
	case "createkey":
		if len(args) < 2 {
			b.sendText(m.Chat.ID, "Usage: /admin createkey <standard|pro>\nCreates a license on the license server only (not stored in bot DB — for manual handoff).")
			return
		}
		tier, ok := parseTierArg(args[1])
		if !ok {
			b.sendText(m.Chat.ID, "Tier must be standard or pro.")
			return
		}
		key := GenLicenseKey()
		exp := time.Now().UTC().Add(b.subscriptionPeriodForTier(tier))
		if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil {
			b.sendText(m.Chat.ID, "License server error: "+err.Error())
			return
		}
		_ = b.sendHTML(m.Chat.ID, fmt.Sprintf(
			"<b>License created</b> (not in bot DB)\n\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s\nMax activations: %d",
			key, tier, exp.Format(time.RFC3339), b.cfg.MaxActivations,
		))
	case "revoke":
		if len(args) < 2 {
			b.sendText(m.Chat.ID, "Usage: /admin revoke LICENSE-KEY\nCalls POST /admin/revoke on license server and deletes bot row if mapped.")
			return
		}
		var key string
		if len(args) == 2 {
			key = strings.TrimSpace(args[1])
		} else {
			key = strings.TrimSpace(strings.Join(args[1:], "-"))
		}
		if key == "" {
			b.sendText(m.Chat.ID, "Empty license key.")
			return
		}
		if err := b.lic.RevokeLicense(ctx, key); err != nil {
			b.sendText(m.Chat.ID, "Revoke failed: "+err.Error())
			return
		}
		tgID, ok, err := b.store.TelegramIDByLicenseKey(ctx, key)
		if err != nil {
			b.sendText(m.Chat.ID, "Revoked on server; bot lookup error: "+err.Error())
			return
		}
		if ok {
			_ = b.store.DeleteSubscription(ctx, tgID)
			b.sendText(m.Chat.ID, fmt.Sprintf("Revoked on server; removed bot row for tg %d.", tgID))
			return
		}
		b.sendText(m.Chat.ID, "Revoked on server (no bot row for this key).")
	default:
		b.sendAdminHelp(m.Chat.ID)
	}
}

func (b *Bot) sendAdminHelp(chatID int64) {
	text := `<b>Admin commands</b> (your Telegram id must be in <code>TELEGRAM_ADMIN_IDS</code>)

/ahelp — this help
/admin — same
/admin stats — subscription &amp; pending invoice counts
/admin user TELEGRAM_USER_ID — license row in bot DB
/admin pending — last pending Crypto Pay invoices
/admin create TELEGRAM_USER_ID standard|pro — new license on server + bot row; DM user if possible
/admin createkey standard|pro — new license on server only (not in bot DB)
/admin droplocal TELEGRAM_USER_ID — delete bot DB only (server not touched)
/admin revoke LICENSE-KEY — POST /admin/revoke + remove bot row if known
`
	_ = b.sendHTML(chatID, text)
}

func parseTierArg(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case tierStandard, "std":
		return tierStandard, true
	case tierPro:
		return tierPro, true
	default:
		return "", false
	}
}

func (b *Bot) readWelcomePhotoLocal() ([]byte, string, bool) {
	if len(embeddedWelcomeJPG) > 0 && looksLikeImageFile(embeddedWelcomeJPG) {
		return embeddedWelcomeJPG, "welcome.jpg", true
	}
	var paths []string
	if p := strings.TrimSpace(b.cfg.WelcomePhotoPath); p != "" {
		paths = append(paths, p)
	}
	paths = append(paths, "welcome_logo.png", "welcome_logo.jpg", "logo.png", "logo.jpg")
	seen := make(map[string]struct{})
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		data, err := os.ReadFile(p)
		if err != nil || len(data) < 24 {
			continue
		}
		if !looksLikeImageFile(data) {
			continue
		}
		return data, filepath.Base(p), true
	}
	return nil, "", false
}

func looksLikeImageFile(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// PNG
	if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return true
	}
	// JPEG
	if data[0] == 0xff && data[1] == 0xd8 {
		return true
	}
	// GIF
	if len(data) >= 6 && (strings.HasPrefix(string(data[:6]), "GIF87a") || strings.HasPrefix(string(data[:6]), "GIF89a")) {
		return true
	}
	return false
}

func (b *Bot) sendWelcome(chatID int64, edit *tgbotapi.CallbackQuery) {
	caption := b.welcomeCaption()
	keyboard := b.mainKeyboard()

	if b.cfg.WelcomePhotoURL != "" {
		if edit != nil {
			_, _ = b.api.Request(tgbotapi.NewCallback(edit.ID, ""))
		}
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(b.cfg.WelcomePhotoURL))
		photo.Caption = caption
		photo.ParseMode = "HTML"
		photo.ReplyMarkup = keyboard
		if _, err := b.api.Send(photo); err != nil {
			b.log.Printf("send photo: %v, falling back to text", err)
			b.sendWelcomeText(chatID, caption, keyboard)
		}
		return
	}
	if pic, fname, ok := b.readWelcomePhotoLocal(); ok {
		if edit != nil {
			_, _ = b.api.Request(tgbotapi.NewCallback(edit.ID, ""))
		}
		photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: fname, Bytes: pic})
		photo.Caption = caption
		photo.ParseMode = "HTML"
		photo.ReplyMarkup = keyboard
		if _, err := b.api.Send(photo); err != nil {
			b.log.Printf("send local photo: %v, falling back to text", err)
			b.sendWelcomeText(chatID, caption, keyboard)
		}
		return
	}
	if edit != nil {
		_, _ = b.api.Request(tgbotapi.NewCallback(edit.ID, ""))
		editMsg := tgbotapi.NewEditMessageText(chatID, edit.Message.MessageID, caption)
		editMsg.ParseMode = "HTML"
		editMsg.ReplyMarkup = &keyboard
		if _, err := b.api.Send(editMsg); err != nil {
			b.sendWelcomeText(chatID, caption, keyboard)
		}
		return
	}
	b.sendWelcomeText(chatID, caption, keyboard)
}

func (b *Bot) sendWelcomeText(chatID int64, caption string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, caption)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	_, _ = b.api.Send(msg)
}

func (b *Bot) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	data := cq.Data
	chatID := cq.Message.Chat.ID
	user := cq.From

	switch {
	case data == "mylicense":
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.replyMyLicense(ctx, chatID, user.ID, user.UserName)
	case strings.HasPrefix(data, "buy:"):
		tier := strings.TrimPrefix(data, "buy:")
		if tier != tierStandard && tier != tierPro {
			_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, "Unknown plan"))
			return
		}
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.startCheckout(ctx, chatID, user.ID, user.UserName, tier)
	default:
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
	}
}

func (b *Bot) replyMyLicense(ctx context.Context, chatID, telegramID int64, username string) {
	sub, err := b.store.GetSubscription(ctx, telegramID)
	if err != nil {
		b.sendText(chatID, "Database error. Try again later.")
		return
	}
	if sub == nil {
		b.sendText(chatID, "No active license on file. Purchase a plan below — after payment the key is sent here automatically.")
		kb := b.mainKeyboard()
		m := tgbotapi.NewMessage(chatID, "Choose a plan:")
		m.ReplyMarkup = kb
		_, _ = b.api.Send(m)
		return
	}
	days := storage.FormatDaysLeft(sub.ExpiresAt)
	tierLabel := "Standard"
	if sub.Tier == tierPro {
		tierLabel = "Pro"
	}
	text := fmt.Sprintf(
		"🔑 <b>Your license</b>\n\nKey:\n<code>%s</code>\n\nPlan: %s\nExpires (UTC): %s\nDays left: <b>%d</b>\n\nEnter the key in SaturX on first run.",
		sub.LicenseKey, tierLabel, sub.ExpiresAt.UTC().Format(time.RFC3339), days,
	)
	_ = b.sendHTMLWithMarkup(chatID, text, b.softwareDownloadKeyboard())
	_ = username
}

func (b *Bot) tierPriceUSDT(tier string) string {
	if tier == tierPro {
		return b.cfg.PriceProUSDT
	}
	return b.cfg.PriceStandardUSDT
}

func (b *Bot) subscriptionPeriodForTier(tier string) time.Duration {
	if tier == tierPro {
		days := b.cfg.PeriodDaysPro
		if days < 1 {
			days = 365
		}
		return time.Duration(days) * 24 * time.Hour
	}
	days := b.cfg.PeriodDays
	if days < 1 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}

func (b *Bot) startCheckout(ctx context.Context, chatID, userID int64, username, tier string) {
	if b.cfg.CryptoPayAPIToken == "" {
		b.sendText(chatID, "Crypto Pay is not configured on the server (missing CRYPTOPAY_API_TOKEN). Ask the admin to add it in @CryptoBot → Crypto Pay.")
		return
	}
	amount := b.tierPriceUSDT(tier)
	desc := "SaturX Standard — 1 month"
	if tier == tierPro {
		desc = "SaturX Pro — 1 year (AI bots upcoming)"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"tg": userID, "t": tier, "ts": time.Now().Unix(),
	})
	inv, err := b.cp.CreateInvoiceUSDT(ctx, amount, desc, string(payload))
	if err != nil {
		b.log.Printf("createInvoice: %v", err)
		b.sendText(chatID, "Could not create invoice. Try again later or contact support.")
		return
	}
	if err := b.store.AddPendingInvoice(ctx, inv.InvoiceID, userID, tier); err != nil {
		b.log.Printf("pending invoice: %v", err)
	}
	payURL := inv.PayURL
	if inv.BotInvoiceURL != "" {
		payURL = inv.BotInvoiceURL
	}
	text := fmt.Sprintf(
		"Invoice <b>#%d</b> created.\nAmount: <b>%s USDT</b> (~$%s)\n\nTap <b>Pay</b> to complete payment in Telegram. After payment your license key is sent here automatically.",
		inv.InvoiceID, inv.Amount, amount,
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("💎 Pay with Crypto", payURL)),
	)
	msg.ReplyMarkup = keyboard
	_, _ = b.api.Send(msg)

	go b.pollUntilPaid(context.Background(), chatID, userID, username, tier, inv.InvoiceID)
}

func (b *Bot) pollUntilPaid(ctx context.Context, chatID, userID int64, username, tier string, invoiceID int64) {
	timeout := time.NewTimer(time.Duration(b.cfg.InvoicePollTimeoutMin) * time.Minute)
	defer timeout.Stop()
	tick := time.NewTicker(time.Duration(b.cfg.InvoicePollSeconds) * time.Second)
	defer tick.Stop()

	un := username
	for {
		select {
		case <-timeout.C:
			_ = b.store.RemovePendingInvoice(ctx, invoiceID)
			return
		case <-tick.C:
			inv, err := b.cp.GetInvoice(ctx, invoiceID)
			if err != nil {
				continue
			}
			switch inv.Status {
			case "paid":
				_ = b.store.RemovePendingInvoice(ctx, invoiceID)
				if err := b.fulfill(ctx, chatID, userID, un, tier); err != nil {
					b.log.Printf("fulfill: %v", err)
					b.sendText(chatID, "Payment received but license issuance failed. Contact support with invoice #"+fmt.Sprint(invoiceID)+".")
				}
				return
			case "expired":
				_ = b.store.RemovePendingInvoice(ctx, invoiceID)
				b.sendText(chatID, "Invoice expired. Choose a plan again to get a new link.")
				return
			}
		}
	}
}

func (b *Bot) fulfill(ctx context.Context, chatID, userID int64, username, tier string) error {
	period := b.subscriptionPeriodForTier(tier)
	now := time.Now().UTC()

	sub, err := b.store.GetSubscription(ctx, userID)
	if err != nil {
		return err
	}

	if sub == nil {
		key := GenLicenseKey()
		exp := now.Add(period)
		if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil {
			return err
		}
		if err := b.store.InsertSubscription(ctx, userID, username, key, tier, exp); err != nil {
			// Concurrent first payments: record may already exist
			sub2, _ := b.store.GetSubscription(ctx, userID)
			if sub2 != nil {
				return b.extendSubscription(ctx, chatID, userID, username, tier, sub2, period, now)
			}
			return err
		}
		text := fmt.Sprintf(
			"✅ <b>Payment confirmed</b>\n\nYour license key:\n<code>%s</code>\n\nValid until (UTC): %s\nActivations: up to %d devices (SaturX)\n\nOpen SaturX → enter the key when asked.",
			key, exp.Format(time.RFC3339), b.cfg.MaxActivations,
		)
		return b.sendHTMLWithMarkup(chatID, text, b.softwareDownloadKeyboard())
	}

	return b.extendSubscription(ctx, chatID, userID, username, tier, sub, period, now)
}

func (b *Bot) extendSubscription(ctx context.Context, chatID, userID int64, username, tier string, sub *storage.Subscription, period time.Duration, now time.Time) error {
	base := sub.ExpiresAt
	if base.Before(now) {
		base = now
	}
	newExp := base.Add(period)
	if err := b.lic.AdminActivate(ctx, sub.LicenseKey, newExp); err != nil {
		return err
	}
	if err := b.store.UpdateExpiryAndTier(ctx, userID, username, tier, newExp); err != nil {
		return err
	}
	text := fmt.Sprintf(
		"✅ <b>Subscription extended</b>\n\nSame license key:\n<code>%s</code>\n\nNew expiry (UTC): %s",
		sub.LicenseKey, newExp.Format(time.RFC3339),
	)
	return b.sendHTMLWithMarkup(chatID, text, b.softwareDownloadKeyboard())
}

func (b *Bot) sendText(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	_, _ = b.api.Send(m)
}

func (b *Bot) sendHTML(chatID int64, html string) error {
	return b.sendHTMLWithMarkup(chatID, html, nil)
}

func (b *Bot) sendHTMLWithMarkup(chatID int64, html string, markup *tgbotapi.InlineKeyboardMarkup) error {
	m := tgbotapi.NewMessage(chatID, html)
	m.ParseMode = "HTML"
	if markup != nil {
		m.ReplyMarkup = markup
	}
	_, err := b.api.Send(m)
	return err
}

// RunReminderScan runs once; call daily from main.
func (b *Bot) RunReminderScan(ctx context.Context) {
	subs, err := b.store.ListSubscriptionsExpiringWithin(ctx, 8)
	if err != nil {
		b.log.Printf("reminder list: %v", err)
		return
	}
	for _, sub := range subs {
		days := storage.FormatDaysLeft(sub.ExpiresAt)
		if days <= 0 {
			continue
		}
		var send bool
		r7, r3, r1 := sub.Remind7d, sub.Remind3d, sub.Remind1d
		switch {
		case days <= 7 && days >= 4 && !r7:
			send = true
			r7 = true
		case days <= 3 && days >= 2 && !r3:
			send = true
			r3 = true
		case days == 1 && !r1:
			send = true
			r1 = true
		}
		if !send {
			continue
		}
		text := fmt.Sprintf(
			"⏰ <b>SaturX license reminder</b>\n\nYour key <code>%s</code> expires in <b>%d</b> day(s) (%s UTC).\nRenew from the bot (/start) to avoid interruption.",
			sub.LicenseKey, days, sub.ExpiresAt.UTC().Format(time.RFC3339),
		)
		_ = b.sendHTML(sub.TelegramUserID, text)
		_ = b.store.SetReminderFlags(ctx, sub.TelegramUserID, r7, r3, r1)
	}
}
