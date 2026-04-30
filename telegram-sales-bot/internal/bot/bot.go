package bot

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	tierTrial    = "trial"
)

type Bot struct {
	cfg    *config.Config
	api    *tgbotapi.BotAPI
	store  *storage.Storage
	lic    *licenseapi.Client
	cp     *cryptopay.Client
	log    *log.Logger
	rateMu sync.Mutex
	last   map[int64]time.Time
}

func New(cfg *config.Config, api *tgbotapi.BotAPI, st *storage.Storage, lic *licenseapi.Client, cp *cryptopay.Client) *Bot {
	return &Bot{cfg: cfg, api: api, store: st, lic: lic, cp: cp, log: log.Default(), last: make(map[int64]time.Time)}
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
	return `🟢 <b>SaturX</b> — Kick chat dashboard in one place.

<b>You get:</b>
• Embedded channel chat + live stream
• Multiple Kick accounts, switch in one click
• Message presets + auto-send
• Per-account SOCKS5 proxy in the dashboard
• Optional viewer tools in the same app
• Automatic license delivery after payment

<b>Plans</b>
• <b>Standard — $29/mo</b>
• <b>Pro — $129/yr</b>
• <b>Demo</b> — one-time trial key for a short test

Use /promo CODE before purchase if you have a discount. Questions? /support.`
}

func (b *Bot) mainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Buy Standard — $29/mo", "buy:"+tierStandard),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐ Buy Pro — $129/yr", "buy:"+tierPro),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧪 Try demo", "trial"),
			tgbotapi.NewInlineKeyboardButtonData("🔑 My license", "mylicense"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 Referral link", "reflink"),
			tgbotapi.NewInlineKeyboardButtonData("📩 Support", "support"),
		),
	)
}

func (b *Bot) softwareDownloadKeyboard() *tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	if u := strings.TrimSpace(b.cfg.SoftwareDownloadURL); u != "" {
		label := strings.TrimSpace(b.cfg.SoftwareDownloadLinkText)
		if label == "" {
			label = "Download SaturX"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("📦 "+label, u)))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("📩 Support", "https://t.me/ssaturx")))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

func (b *Bot) renewKeyboard() *tgbotapi.InlineKeyboardMarkup {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Renew Standard", "buy:"+tierStandard),
			tgbotapi.NewInlineKeyboardButtonData("⭐ Renew Pro", "buy:"+tierPro),
		),
	)
	return &kb
}

func (b *Bot) HandleUpdate(ctx context.Context, u tgbotapi.Update) {
	switch {
	case u.Message != nil && u.Message.IsCommand():
		if !b.allowAction(u.Message.From.ID, b.cfg.IsAdmin(u.Message.From.ID)) {
			b.sendText(u.Message.Chat.ID, "Too many requests. Try again in a second.")
			return
		}
		b.handleCommand(ctx, u.Message)
	case u.CallbackQuery != nil:
		if !b.allowAction(u.CallbackQuery.From.ID, b.cfg.IsAdmin(u.CallbackQuery.From.ID)) {
			_, _ = b.api.Request(tgbotapi.NewCallback(u.CallbackQuery.ID, "Try again in a second"))
			return
		}
		b.handleCallback(ctx, u.CallbackQuery)
	}
}

func (b *Bot) allowAction(userID int64, admin bool) bool {
	if admin {
		return true
	}
	b.rateMu.Lock()
	defer b.rateMu.Unlock()
	now := time.Now()
	if last, ok := b.last[userID]; ok && now.Sub(last) < time.Second {
		return false
	}
	b.last[userID] = now
	return true
}

func (b *Bot) handleCommand(ctx context.Context, m *tgbotapi.Message) {
	b.registerUserFromStart(ctx, m)
	userID := m.From.ID
	switch m.Command() {
	case "start":
		b.event(ctx, &userID, "start", map[string]interface{}{"args": m.CommandArguments()})
		b.sendWelcome(m.Chat.ID, nil)
	case "promo":
		b.handlePromoCommand(ctx, m)
	case "support":
		b.sendSupport(m.Chat.ID)
	case "admin", "ahelp":
		b.handleAdmin(ctx, m)
	default:
		msg := tgbotapi.NewMessage(m.Chat.ID, "Use /start for plans, /promo CODE for discounts, or /support for the manager.")
		_, _ = b.api.Send(msg)
	}
}

func (b *Bot) registerUserFromStart(ctx context.Context, m *tgbotapi.Message) {
	if m.From == nil {
		return
	}
	var inviter *int64
	if m.Command() == "start" {
		arg := strings.TrimSpace(m.CommandArguments())
		if strings.HasPrefix(arg, "ref_") {
			id, err := strconv.ParseInt(strings.TrimPrefix(arg, "ref_"), 10, 64)
			if err == nil && id > 0 && id != m.From.ID {
				inviter = &id
			}
		}
	}
	registered, err := b.store.UpsertUser(ctx, m.From.ID, m.From.UserName, inviter)
	if err != nil {
		b.log.Printf("upsert user %d: %v", m.From.ID, err)
		return
	}
	if registered && inviter != nil {
		b.event(ctx, &m.From.ID, "referral_registered", map[string]interface{}{"inviter_id": *inviter})
	}
}

func (b *Bot) handlePromoCommand(ctx context.Context, m *tgbotapi.Message) {
	code := storage.NormalizePromoCode(m.CommandArguments())
	if code == "" {
		b.sendText(m.Chat.ID, "Usage: /promo CODE")
		return
	}
	p, err := b.store.GetPromoCode(ctx, code)
	if err != nil {
		b.sendText(m.Chat.ID, "Database error. Try again later.")
		return
	}
	if p == nil {
		b.sendText(m.Chat.ID, "Promo code not found.")
		return
	}
	if ok, reason := p.IsValidFor(p.Tier); !ok && reason != "Promo code does not match this plan." {
		b.sendText(m.Chat.ID, reason)
		return
	}
	used, err := b.store.HasPromoRedemption(ctx, code, m.From.ID)
	if err != nil {
		b.sendText(m.Chat.ID, "Database error. Try again later.")
		return
	}
	if used {
		b.sendText(m.Chat.ID, "You already used this promo code.")
		return
	}
	if err := b.store.SetActivePromo(ctx, m.From.ID, code); err != nil {
		b.sendText(m.Chat.ID, "Could not apply promo code. Try again later.")
		return
	}
	b.event(ctx, &m.From.ID, "promo_applied", map[string]interface{}{"code": code, "percent_off": p.PercentOff, "tier": p.Tier})
	b.sendHTML(m.Chat.ID, fmt.Sprintf("✅ Promo <code>%s</code> applied: <b>%d%% off</b> for %s. Choose a plan with /start.", code, p.PercentOff, p.Tier))
}

func (b *Bot) handleAdmin(ctx context.Context, m *tgbotapi.Message) {
	if !b.cfg.IsAdmin(m.From.ID) {
		msg := tgbotapi.NewMessage(m.Chat.ID, "Use /start for plans. /support for the manager.")
		_, _ = b.api.Send(msg)
		return
	}
	adminID := m.From.ID
	b.event(ctx, &adminID, "admin_command", map[string]interface{}{"command": m.Command(), "args": m.CommandArguments()})
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
		b.adminStats(ctx, m.Chat.ID)
	case "user":
		b.adminUser(ctx, m.Chat.ID, args)
	case "pending":
		b.adminPending(ctx, m.Chat.ID)
	case "droplocal":
		b.adminDropLocal(ctx, m.Chat.ID, args)
	case "create":
		b.adminCreate(ctx, m.Chat.ID, args)
	case "createkey":
		b.adminCreateKey(ctx, m.Chat.ID, args)
	case "revoke":
		b.adminRevoke(ctx, m.Chat.ID, args)
	case "promo":
		b.adminPromo(ctx, m.Chat.ID, args)
	case "broadcast":
		msg := strings.TrimSpace(strings.TrimPrefix(m.CommandArguments(), "broadcast"))
		b.adminBroadcast(ctx, m.Chat.ID, msg, false)
	case "broadcast_subscribers":
		msg := strings.TrimSpace(strings.TrimPrefix(m.CommandArguments(), "broadcast_subscribers"))
		b.adminBroadcast(ctx, m.Chat.ID, msg, true)
	case "backup":
		b.adminBackup(ctx, m.Chat.ID)
	default:
		b.sendAdminHelp(m.Chat.ID)
	}
}

func (b *Bot) adminStats(ctx context.Context, chatID int64) {
	st, err := b.store.Stats(ctx)
	if err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	var rev strings.Builder
	if len(st.RevenueByTier) == 0 {
		rev.WriteString("none")
	} else {
		for tier, value := range st.RevenueByTier {
			rev.WriteString(fmt.Sprintf("%s: %s USDT\n", tier, storage.FormatUSDT(value)))
		}
	}
	b.sendHTML(chatID, fmt.Sprintf(
		"<b>Stats</b>\nUsers total: %d\nSubscriptions total: %d\nActive subscriptions: %d\nPending invoices: %d\nPaid invoices: %d\nExpired invoices: %d\nTrials claimed: %d\nEvents last 24h: %d\n\n<b>Revenue estimate</b>\n%s",
		st.UsersTotal, st.SubscriptionsTotal, st.ActiveSubscriptions, st.PendingInvoices, st.PaidInvoices, st.ExpiredInvoices, st.TrialsClaimed, st.EventsLast24h, strings.TrimSpace(rev.String()),
	))
}

func (b *Bot) adminUser(ctx context.Context, chatID int64, args []string) {
	if len(args) < 2 {
		b.sendText(chatID, "Usage: /admin user <telegram_user_id>")
		return
	}
	uid, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		b.sendText(chatID, "Invalid user id.")
		return
	}
	sub, err := b.store.GetSubscription(ctx, uid)
	if err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	if sub == nil {
		b.sendText(chatID, "No subscription for this Telegram id.")
		return
	}
	days := storage.FormatDaysLeft(sub.ExpiresAt)
	un := sub.Username
	if un != "" {
		un = "@" + un
	} else {
		un = "(no username)"
	}
	b.sendHTML(chatID, fmt.Sprintf(
		"<b>User</b> <code>%d</code> %s\n<b>Key</b> <code>%s</code>\n<b>Tier</b> %s\n<b>Expires</b> %s UTC\n<b>Days left</b> %d",
		sub.TelegramUserID, un, sub.LicenseKey, sub.Tier, sub.ExpiresAt.UTC().Format(time.RFC3339), days,
	))
}

func (b *Bot) adminPending(ctx context.Context, chatID int64) {
	rows, err := b.store.ListPendingInvoices(ctx, 15)
	if err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	if len(rows) == 0 {
		b.sendText(chatID, "No pending invoices.")
		return
	}
	var sb strings.Builder
	sb.WriteString("<b>Pending invoices</b>\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("inv <code>%d</code> tg <code>%d</code> %s %s\n", r.InvoiceID, r.TelegramUserID, r.Tier, r.Status))
	}
	_ = b.sendHTML(chatID, sb.String())
}

func (b *Bot) adminDropLocal(ctx context.Context, chatID int64, args []string) {
	if len(args) < 2 {
		b.sendText(chatID, "Usage: /admin droplocal <telegram_user_id>")
		return
	}
	uid, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		b.sendText(chatID, "Invalid user id.")
		return
	}
	if err := b.store.DeleteSubscription(ctx, uid); err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	b.sendText(chatID, fmt.Sprintf("Removed bot records for tg %d (license server not changed).", uid))
}

func (b *Bot) adminCreate(ctx context.Context, chatID int64, args []string) {
	if len(args) < 3 {
		b.sendText(chatID, "Usage: /admin create <telegram_user_id> <standard|pro>")
		return
	}
	uid, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		b.sendText(chatID, "Invalid telegram_user_id.")
		return
	}
	tier, ok := parseTierArg(args[2])
	if !ok || tier == tierTrial {
		b.sendText(chatID, "Tier must be standard or pro.")
		return
	}
	sub, err := b.store.GetSubscription(ctx, uid)
	if err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	if sub != nil {
		b.sendText(chatID, fmt.Sprintf("User %d already has a bot subscription.", uid))
		return
	}
	key := GenLicenseKey()
	exp := time.Now().UTC().Add(b.subscriptionPeriodForTier(tier))
	if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil {
		b.sendText(chatID, "License server error: "+err.Error())
		return
	}
	if err := b.store.InsertSubscription(ctx, uid, "", key, tier, exp); err != nil {
		b.sendText(chatID, "Created on server but bot DB failed: "+err.Error()+"\nKey: "+key)
		return
	}
	userHTML := licenseMessage("Your SaturX license", key, tier, exp, b.cfg.MaxActivations)
	if err := b.sendHTMLWithMarkup(uid, userHTML, b.softwareDownloadKeyboard()); err != nil {
		b.log.Printf("admin create: DM user %d: %v", uid, err)
		b.sendHTML(chatID, fmt.Sprintf("<b>Created</b> for <code>%d</code> (could not DM).\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s", uid, key, tier, exp.Format(time.RFC3339)))
		return
	}
	b.sendHTML(chatID, fmt.Sprintf("<b>Created</b> for <code>%d</code> — DM sent.\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s", uid, key, tier, exp.Format(time.RFC3339)))
}

func (b *Bot) adminCreateKey(ctx context.Context, chatID int64, args []string) {
	if len(args) < 2 {
		b.sendText(chatID, "Usage: /admin createkey <standard|pro>")
		return
	}
	tier, ok := parseTierArg(args[1])
	if !ok || tier == tierTrial {
		b.sendText(chatID, "Tier must be standard or pro.")
		return
	}
	key := GenLicenseKey()
	exp := time.Now().UTC().Add(b.subscriptionPeriodForTier(tier))
	if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil {
		b.sendText(chatID, "License server error: "+err.Error())
		return
	}
	_ = b.sendHTML(chatID, fmt.Sprintf("<b>License created</b> (not in bot DB)\n\nKey: <code>%s</code>\nTier: %s\nExpires UTC: %s\nMax activations: %d", key, tier, exp.Format(time.RFC3339), b.cfg.MaxActivations))
}

func (b *Bot) adminRevoke(ctx context.Context, chatID int64, args []string) {
	if len(args) < 2 {
		b.sendText(chatID, "Usage: /admin revoke LICENSE-KEY")
		return
	}
	key := strings.TrimSpace(strings.Join(args[1:], "-"))
	if key == "" {
		b.sendText(chatID, "Empty license key.")
		return
	}
	if err := b.lic.RevokeLicense(ctx, key); err != nil {
		b.sendText(chatID, "Revoke failed: "+err.Error())
		return
	}
	tgID, ok, err := b.store.TelegramIDByLicenseKey(ctx, key)
	if err != nil {
		b.sendText(chatID, "Revoked on server; bot lookup error: "+err.Error())
		return
	}
	if ok {
		_ = b.store.DeleteSubscription(ctx, tgID)
		b.sendText(chatID, fmt.Sprintf("Revoked on server; removed bot row for tg %d.", tgID))
		return
	}
	b.sendText(chatID, "Revoked on server (no bot row for this key).")
}

func (b *Bot) adminPromo(ctx context.Context, chatID int64, args []string) {
	if len(args) < 2 {
		b.sendText(chatID, "Usage: /admin promo create|list|disable ...")
		return
	}
	switch args[1] {
	case "create":
		if len(args) < 7 {
			b.sendText(chatID, "Usage: /admin promo create <CODE> <percent_off> <standard|pro|any> <max_uses> <days_valid>")
			return
		}
		code := storage.NormalizePromoCode(args[2])
		percent, err1 := strconv.Atoi(args[3])
		tier := strings.ToLower(args[4])
		maxUses, err2 := strconv.Atoi(args[5])
		days, err3 := strconv.Atoi(args[6])
		if code == "" || err1 != nil || err2 != nil || err3 != nil || percent < 1 || percent > 90 || maxUses <= 0 || (tier != tierStandard && tier != tierPro && tier != "any") {
			b.sendText(chatID, "Invalid promo. percent_off must be 1-90, tier standard|pro|any, max_uses > 0.")
			return
		}
		var exp *time.Time
		if days > 0 {
			t := time.Now().UTC().AddDate(0, 0, days)
			exp = &t
		}
		if err := b.store.CreatePromoCode(ctx, code, percent, tier, maxUses, exp); err != nil {
			b.sendText(chatID, "DB error: "+err.Error())
			return
		}
		b.sendHTML(chatID, fmt.Sprintf("Promo <code>%s</code> created: %d%% off, tier %s, max uses %d.", code, percent, tier, maxUses))
	case "list":
		rows, err := b.store.ListPromoCodes(ctx, 30)
		if err != nil {
			b.sendText(chatID, "DB error: "+err.Error())
			return
		}
		if len(rows) == 0 {
			b.sendText(chatID, "No promo codes.")
			return
		}
		var sb strings.Builder
		sb.WriteString("<b>Promo codes</b>\n")
		for _, p := range rows {
			exp := "never"
			if p.ExpiresAt.Valid {
				exp = p.ExpiresAt.String
			}
			active := "off"
			if p.Active {
				active = "on"
			}
			sb.WriteString(fmt.Sprintf("<code>%s</code> %d%% %s %d/%d exp %s %s\n", p.Code, p.PercentOff, p.Tier, p.UsedCount, p.MaxUses, exp, active))
		}
		b.sendHTML(chatID, sb.String())
	case "disable":
		if len(args) < 3 {
			b.sendText(chatID, "Usage: /admin promo disable <CODE>")
			return
		}
		if err := b.store.DisablePromoCode(ctx, args[2]); err != nil {
			b.sendText(chatID, "DB error: "+err.Error())
			return
		}
		b.sendHTML(chatID, fmt.Sprintf("Promo <code>%s</code> disabled.", storage.NormalizePromoCode(args[2])))
	default:
		b.sendText(chatID, "Usage: /admin promo create|list|disable ...")
	}
}

func (b *Bot) adminBroadcast(ctx context.Context, chatID int64, text string, subscribersOnly bool) {
	if text == "" {
		if subscribersOnly {
			b.sendText(chatID, "Usage: /admin broadcast_subscribers <message>")
		} else {
			b.sendText(chatID, "Usage: /admin broadcast <message>")
		}
		return
	}
	var ids []int64
	var err error
	if subscribersOnly {
		ids, err = b.store.ListSubscriberIDs(ctx)
	} else {
		ids, err = b.store.ListAllUserIDs(ctx)
	}
	if err != nil {
		b.sendText(chatID, "DB error: "+err.Error())
		return
	}
	success, failed := 0, 0
	for _, id := range ids {
		msg := tgbotapi.NewMessage(id, text)
		if _, err := b.api.Send(msg); err != nil {
			failed++
		} else {
			success++
		}
		time.Sleep(80 * time.Millisecond)
	}
	b.sendText(chatID, fmt.Sprintf("Broadcast complete. Success: %d, failed: %d.", success, failed))
}

func (b *Bot) adminBackup(ctx context.Context, chatID int64) {
	src := b.store.Path()
	if src == "" {
		b.sendText(chatID, "Storage path is empty.")
		return
	}
	abs, _ := filepath.Abs(src)
	dst := abs + ".backup." + time.Now().UTC().Format("20060102-150405") + ".db"
	if err := copyFile(abs, dst); err != nil {
		b.sendText(chatID, "Backup failed: "+err.Error())
		return
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(dst))
	doc.Caption = "SQLite backup: " + filepath.Base(dst)
	if _, err := b.api.Send(doc); err != nil {
		b.sendText(chatID, "Backup saved: "+dst+"\nTelegram upload failed: "+err.Error())
		return
	}
	b.sendText(chatID, "Backup created and sent: "+dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func (b *Bot) sendAdminHelp(chatID int64) {
	text := `<b>Admin commands</b>

/ahelp — this help
/admin stats — users, subscriptions, invoices, trials, revenue, events
/admin user TELEGRAM_USER_ID — license row in bot DB
/admin pending — recent pending/paid invoices
/admin create TELEGRAM_USER_ID standard|pro — create license on server + bot DB
/admin createkey standard|pro — create license on server only
/admin droplocal TELEGRAM_USER_ID — delete bot DB only
/admin revoke LICENSE-KEY — POST /admin/revoke + remove bot row if known
/admin promo create CODE percent standard|pro|any max_uses days_valid
/admin promo list
/admin promo disable CODE
/admin broadcast MESSAGE — all known users
/admin broadcast_subscribers MESSAGE — users with subscription
/admin backup — copy SQLite DB and send it back`
	_ = b.sendHTML(chatID, text)
}

func parseTierArg(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case tierStandard, "std":
		return tierStandard, true
	case tierPro:
		return tierPro, true
	case tierTrial:
		return tierTrial, true
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
		if err != nil || len(data) < 24 || !looksLikeImageFile(data) {
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
	if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return true
	}
	if data[0] == 0xff && data[1] == 0xd8 {
		return true
	}
	return len(data) >= 6 && (strings.HasPrefix(string(data[:6]), "GIF87a") || strings.HasPrefix(string(data[:6]), "GIF89a"))
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
	_, _ = b.store.UpsertUser(ctx, user.ID, user.UserName, nil)

	switch {
	case data == "mylicense":
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.replyMyLicense(ctx, chatID, user.ID, user.UserName)
	case data == "trial":
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.claimTrial(ctx, chatID, user.ID, user.UserName)
	case data == "reflink":
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.replyReferralLink(chatID, user.ID)
	case data == "support":
		_, _ = b.api.Request(tgbotapi.NewCallback(cq.ID, ""))
		b.sendSupport(chatID)
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

func (b *Bot) sendSupport(chatID int64) {
	b.sendHTML(chatID, `📩 <b>Support</b>

Manager: <a href="https://t.me/ssaturx">@ssaturx</a>

Write in Telegram for payment issues, license help, or product questions.`)
}

func (b *Bot) replyReferralLink(chatID, userID int64) {
	username := b.api.Self.UserName
	link := fmt.Sprintf("https://t.me/%s?start=ref_%d", username, userID)
	text := fmt.Sprintf("🔗 <b>Your referral link</b>\n\n<code>%s</code>\n\nShare it with a buyer. After their first successful payment, your current license gets <b>%d bonus day(s)</b>.", link, b.cfg.ReferralBonusDays)
	b.sendHTML(chatID, text)
}

func (b *Bot) replyMyLicense(ctx context.Context, chatID, telegramID int64, username string) {
	sub, err := b.store.GetSubscription(ctx, telegramID)
	if err != nil {
		b.sendText(chatID, "Database error. Try again later.")
		return
	}
	if sub == nil {
		b.sendText(chatID, "No active license on file. Purchase a plan or try the one-time demo below.")
		kb := b.mainKeyboard()
		m := tgbotapi.NewMessage(chatID, "Choose an option:")
		m.ReplyMarkup = kb
		_, _ = b.api.Send(m)
		return
	}
	days := storage.FormatDaysLeft(sub.ExpiresAt)
	text := fmt.Sprintf("🔑 <b>Your license</b>\n\nKey:\n<code>%s</code>\n\nPlan: %s\nExpires (UTC): %s\nDays left: <b>%d</b>\n\nOpen SaturX, paste the key on first run, then keep this chat for renewal reminders.", sub.LicenseKey, planLabel(sub.Tier), sub.ExpiresAt.UTC().Format(time.RFC3339), days)
	_ = b.sendHTMLWithMarkup(chatID, text, b.softwareDownloadKeyboard())
	_ = username
}

func (b *Bot) claimTrial(ctx context.Context, chatID, userID int64, username string) {
	used, err := b.store.HasTrialClaim(ctx, userID)
	if err != nil {
		b.sendText(chatID, "Database error. Try again later.")
		return
	}
	if used {
		b.sendText(chatID, "Demo is one-time only for each Telegram account. Use My license or choose a plan to continue.")
		return
	}
	now := time.Now().UTC()
	exp := now.Add(time.Duration(b.cfg.TrialPeriodHours) * time.Hour)
	sub, err := b.store.GetSubscription(ctx, userID)
	if err != nil {
		b.sendText(chatID, "Database error. Try again later.")
		return
	}
	if sub != nil && sub.ExpiresAt.After(now) && sub.Tier != tierTrial {
		b.sendText(chatID, "You already have an active paid license. Demo is not needed.")
		return
	}
	key := GenLicenseKey()
	if sub != nil {
		key = sub.LicenseKey
		if err := b.lic.AdminActivate(ctx, key, exp); err != nil {
			b.sendText(chatID, "Could not activate demo. Contact support.")
			return
		}
		if err := b.store.UpdateExpiryAndTier(ctx, userID, username, tierTrial, exp); err != nil {
			b.sendText(chatID, "Demo created but local DB failed. Contact support.")
			return
		}
	} else {
		if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.TrialMaxActivations); err != nil {
			b.sendText(chatID, "Could not create demo key. Contact support.")
			return
		}
		if err := b.store.InsertSubscription(ctx, userID, username, key, tierTrial, exp); err != nil {
			b.sendText(chatID, "Demo created but local DB failed. Contact support.")
			return
		}
	}
	if err := b.store.InsertTrialClaim(ctx, userID, key, exp); err != nil {
		b.sendText(chatID, "Demo already claimed.")
		return
	}
	b.event(ctx, &userID, "trial_claimed", map[string]interface{}{"expires_at": exp.Format(time.RFC3339)})
	text := fmt.Sprintf("🧪 <b>Your SaturX demo key</b>\n\nKey:\n<code>%s</code>\n\nPlan: Demo\nExpires (UTC): %s\nActivations: up to %d device.\n\nThis demo is one-time only. Open SaturX and paste the key on first run.", key, exp.Format(time.RFC3339), b.cfg.TrialMaxActivations)
	_ = b.sendHTMLWithMarkup(chatID, text, b.softwareDownloadKeyboard())
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
	b.event(ctx, &userID, "buy_clicked", map[string]interface{}{"tier": tier})
	if b.cfg.CryptoPayAPIToken == "" {
		b.sendText(chatID, "Crypto Pay is not configured on the server. Contact support.")
		return
	}
	baseAmount := b.tierPriceUSDT(tier)
	finalAmount, promoCode, percentOff, promoNote := b.applyActivePromo(ctx, userID, tier, baseAmount)
	desc := "SaturX Standard — 1 month"
	if tier == tierPro {
		desc = "SaturX Pro — 1 year"
	}
	if promoCode != "" {
		desc += " — promo " + promoCode
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"tg": userID, "t": tier, "promo": promoCode, "ts": time.Now().Unix(),
	})
	inv, err := b.cp.CreateInvoiceUSDT(ctx, finalAmount, desc, string(payload))
	if err != nil {
		b.log.Printf("create invoice user %d tier %s: %v", userID, tier, err)
		b.sendText(chatID, "Could not create invoice. Try again later or contact support.")
		return
	}
	if err := b.store.AddPendingInvoice(ctx, storage.PendingInvoice{
		InvoiceID:       inv.InvoiceID,
		TelegramUserID:  userID,
		ChatID:          chatID,
		Username:        username,
		Tier:            tier,
		AmountUSDT:      baseAmount,
		FinalAmountUSDT: finalAmount,
		PromoCode:       promoCode,
	}); err != nil {
		b.log.Printf("pending invoice %d: %v", inv.InvoiceID, err)
	}
	b.event(ctx, &userID, "invoice_created", map[string]interface{}{"invoice_id": inv.InvoiceID, "tier": tier, "amount": finalAmount, "promo": promoCode})
	payURL := inv.PayURL
	if inv.BotInvoiceURL != "" {
		payURL = inv.BotInvoiceURL
	}
	priceLine := fmt.Sprintf("Amount: <b>%s USDT</b>", finalAmount)
	if promoCode != "" {
		priceLine = fmt.Sprintf("Price: <s>%s USDT</s> → <b>%s USDT</b>\nPromo: <code>%s</code> (%d%% off)", baseAmount, finalAmount, promoCode, percentOff)
	} else if promoNote != "" {
		priceLine += "\n" + promoNote
	}
	text := fmt.Sprintf("Invoice <b>#%d</b> created.\nPlan: <b>%s</b>\n%s\n\nTap <b>Pay</b> to complete payment in Telegram. After payment your license key is sent here automatically.", inv.InvoiceID, planLabel(tier), priceLine)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonURL("💎 Pay with Crypto", payURL)),
	)
	_, _ = b.api.Send(msg)
	go b.pollUntilPaid(context.Background(), inv.InvoiceID)
}

func (b *Bot) applyActivePromo(ctx context.Context, userID int64, tier, baseAmount string) (string, string, int, string) {
	code, ok, err := b.store.GetActivePromo(ctx, userID)
	if err != nil || !ok {
		return baseAmount, "", 0, ""
	}
	p, err := b.store.GetPromoCode(ctx, code)
	if err != nil || p == nil {
		_ = b.store.ClearActivePromo(ctx, userID)
		return baseAmount, "", 0, "Saved promo code is no longer available."
	}
	used, _ := b.store.HasPromoRedemption(ctx, code, userID)
	if used {
		_ = b.store.ClearActivePromo(ctx, userID)
		return baseAmount, "", 0, "Promo code was already used."
	}
	if ok, reason := p.IsValidFor(tier); !ok {
		if reason != "Promo code does not match this plan." {
			_ = b.store.ClearActivePromo(ctx, userID)
		}
		return baseAmount, "", 0, reason
	}
	base, err := strconv.ParseFloat(baseAmount, 64)
	if err != nil {
		return baseAmount, "", 0, ""
	}
	final := base * float64(100-p.PercentOff) / 100
	return storage.FormatUSDT(final), p.Code, p.PercentOff, ""
}

func (b *Bot) pollUntilPaid(ctx context.Context, invoiceID int64) {
	timeout := time.NewTimer(time.Duration(b.cfg.InvoicePollTimeoutMin) * time.Minute)
	defer timeout.Stop()
	tick := time.NewTicker(time.Duration(b.cfg.InvoicePollSeconds) * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-timeout.C:
			_ = b.store.MarkInvoiceExpired(ctx, invoiceID)
			if inv, _ := b.store.GetPendingInvoice(ctx, invoiceID); inv != nil {
				b.event(ctx, &inv.TelegramUserID, "invoice_expired", map[string]interface{}{"invoice_id": invoiceID})
			}
			return
		case <-tick.C:
			inv, err := b.cp.GetInvoice(ctx, invoiceID)
			if err != nil {
				continue
			}
			switch inv.Status {
			case "paid":
				b.processPaidInvoice(ctx, invoiceID)
				return
			case "expired":
				_ = b.store.MarkInvoiceExpired(ctx, invoiceID)
				local, _ := b.store.GetPendingInvoice(ctx, invoiceID)
				if local != nil {
					b.event(ctx, &local.TelegramUserID, "invoice_expired", map[string]interface{}{"invoice_id": invoiceID})
					b.sendText(local.ChatID, "Invoice expired. Choose a plan again to get a new link.")
				}
				return
			}
		}
	}
}

func (b *Bot) processPaidInvoice(ctx context.Context, invoiceID int64) {
	inv, claimed, err := b.store.TryMarkInvoicePaidForFulfillment(ctx, invoiceID)
	if err != nil {
		b.log.Printf("invoice %d claim: %v", invoiceID, err)
		return
	}
	if inv == nil || !claimed {
		return
	}
	b.event(ctx, &inv.TelegramUserID, "invoice_paid", map[string]interface{}{"invoice_id": invoiceID, "tier": inv.Tier})
	key, action, err := b.fulfillInvoice(ctx, inv)
	if err != nil {
		_ = b.store.ClearInvoiceProcessing(ctx, invoiceID)
		b.log.Printf("fulfill invoice %d user %d: %v", invoiceID, inv.TelegramUserID, err)
		b.sendText(inv.ChatID, "Payment received but license issuance failed. Contact support with invoice #"+fmt.Sprint(invoiceID)+".")
		return
	}
	if err := b.store.MarkInvoiceFulfilled(ctx, invoiceID, key); err != nil {
		b.log.Printf("invoice %d mark fulfilled key %s: %v", invoiceID, storage.MaskLicenseKey(key), err)
	}
	if inv.PromoCode != "" {
		if err := b.store.RedeemPromo(ctx, inv.PromoCode, inv.TelegramUserID); err != nil {
			b.log.Printf("promo redeem invoice %d code %s: %v", invoiceID, inv.PromoCode, err)
		}
	}
	b.rewardReferral(ctx, inv.TelegramUserID, invoiceID)
	eventType := "license_issued"
	if action == "renewed" {
		eventType = "license_renewed"
	}
	b.event(ctx, &inv.TelegramUserID, eventType, map[string]interface{}{"invoice_id": invoiceID, "tier": inv.Tier, "key": storage.MaskLicenseKey(key)})
}

func (b *Bot) fulfillInvoice(ctx context.Context, inv *storage.PendingInvoice) (string, string, error) {
	period := b.subscriptionPeriodForTier(inv.Tier)
	now := time.Now().UTC()
	sub, err := b.store.GetSubscription(ctx, inv.TelegramUserID)
	if err != nil {
		return "", "", err
	}
	if sub == nil {
		key := GenLicenseKey()
		if inv.LicenseKey.Valid && inv.LicenseKey.String != "" {
			key = inv.LicenseKey.String
		}
		exp := now.Add(period)
		_ = b.store.SetInvoiceLicenseKey(ctx, inv.InvoiceID, key)
		if err := b.lic.CreateLicense(ctx, key, exp, b.cfg.MaxActivations); err != nil && !looksLikeAlreadyExists(err) {
			return "", "", err
		}
		if err := b.store.InsertSubscription(ctx, inv.TelegramUserID, inv.Username, key, inv.Tier, exp); err != nil {
			sub2, _ := b.store.GetSubscription(ctx, inv.TelegramUserID)
			if sub2 != nil {
				return b.extendSubscription(ctx, inv, sub2, period, now)
			}
			return "", "", err
		}
		text := licenseMessage("Payment confirmed", key, inv.Tier, exp, b.cfg.MaxActivations)
		return key, "issued", b.sendHTMLWithMarkup(inv.ChatID, text, b.softwareDownloadKeyboard())
	}
	return b.extendSubscription(ctx, inv, sub, period, now)
}

func (b *Bot) extendSubscription(ctx context.Context, inv *storage.PendingInvoice, sub *storage.Subscription, period time.Duration, now time.Time) (string, string, error) {
	base := sub.ExpiresAt
	if base.Before(now) {
		base = now
	}
	newExp := base.Add(period)
	_ = b.store.SetInvoiceLicenseKey(ctx, inv.InvoiceID, sub.LicenseKey)
	if err := b.lic.AdminActivate(ctx, sub.LicenseKey, newExp); err != nil {
		return "", "", err
	}
	if err := b.store.UpdateExpiryAndTier(ctx, inv.TelegramUserID, inv.Username, inv.Tier, newExp); err != nil {
		return "", "", err
	}
	text := fmt.Sprintf("✅ <b>Subscription extended</b>\n\nKey:\n<code>%s</code>\n\nPlan: %s\nNew expiry (UTC): %s\n\nOpen SaturX and keep using the same key.", sub.LicenseKey, planLabel(inv.Tier), newExp.Format(time.RFC3339))
	return sub.LicenseKey, "renewed", b.sendHTMLWithMarkup(inv.ChatID, text, b.softwareDownloadKeyboard())
}

func (b *Bot) rewardReferral(ctx context.Context, buyerID, invoiceID int64) {
	inviterID, ok, err := b.store.GetInviterID(ctx, buyerID)
	if err != nil || !ok || inviterID == buyerID {
		return
	}
	exists, err := b.store.ReferralRewardExists(ctx, buyerID)
	if err != nil || exists {
		return
	}
	bonusDays := b.cfg.ReferralBonusDays
	if bonusDays <= 0 {
		bonusDays = 7
	}
	sub, err := b.store.GetSubscription(ctx, inviterID)
	if err != nil {
		return
	}
	if sub == nil {
		b.sendText(inviterID, fmt.Sprintf("🎉 Your referral made a purchase. Buy a plan first, then contact support to apply your %d bonus day(s).", bonusDays))
		return
	}
	now := time.Now().UTC()
	base := sub.ExpiresAt
	if base.Before(now) {
		base = now
	}
	newExp := base.AddDate(0, 0, bonusDays)
	if err := b.lic.AdminActivate(ctx, sub.LicenseKey, newExp); err != nil {
		b.log.Printf("referral activate inviter %d: %v", inviterID, err)
		return
	}
	if err := b.store.UpdateExpiryAndTier(ctx, inviterID, sub.Username, sub.Tier, newExp); err != nil {
		b.log.Printf("referral db inviter %d: %v", inviterID, err)
		return
	}
	if err := b.store.InsertReferralReward(ctx, inviterID, buyerID, invoiceID, bonusDays); err != nil {
		return
	}
	b.event(ctx, &inviterID, "referral_rewarded", map[string]interface{}{"invited_user_id": buyerID, "invoice_id": invoiceID, "bonus_days": bonusDays})
	b.sendHTML(inviterID, fmt.Sprintf("🎉 <b>Referral bonus applied</b>\n\nYour invited user paid successfully. We added <b>%d day(s)</b> to your license.\nNew expiry (UTC): %s", bonusDays, newExp.Format(time.RFC3339)))
}

func (b *Bot) ResumeOpenInvoices(ctx context.Context) {
	if err := b.store.ResetOpenInvoiceProcessing(ctx); err != nil {
		b.log.Printf("reset invoice processing: %v", err)
	}
	invoices, err := b.store.ListInvoicesToResume(ctx)
	if err != nil {
		b.log.Printf("resume invoices: %v", err)
		return
	}
	for _, inv := range invoices {
		inv := inv
		if inv.Status == "paid" {
			go b.processPaidInvoice(context.Background(), inv.InvoiceID)
			continue
		}
		go b.pollUntilPaid(context.Background(), inv.InvoiceID)
	}
}

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
		text := fmt.Sprintf("⏰ <b>SaturX license reminder</b>\n\nYour key <code>%s</code> expires in <b>%d</b> day(s) (%s UTC).\nRenew now to avoid interruption.", sub.LicenseKey, days, sub.ExpiresAt.UTC().Format(time.RFC3339))
		_ = b.sendHTMLWithMarkup(sub.TelegramUserID, text, b.renewKeyboard())
		_ = b.store.SetReminderFlags(ctx, sub.TelegramUserID, r7, r3, r1)
	}
}

func (b *Bot) event(ctx context.Context, telegramUserID *int64, eventType string, payload map[string]interface{}) {
	var raw string
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			raw = string(b)
		}
	}
	if err := b.store.RecordEvent(ctx, telegramUserID, eventType, raw); err != nil {
		b.log.Printf("event %s: %v", eventType, err)
	}
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

func licenseMessage(title, key, tier string, exp time.Time, maxActivations int) string {
	return fmt.Sprintf("✅ <b>%s</b>\n\nLicense key:\n<code>%s</code>\n\nPlan: %s\nExpires (UTC): %s\nActivations: up to %d device(s).\n\nOpen SaturX, paste the key on first run, and keep this chat for renewals and support.", title, key, planLabel(tier), exp.Format(time.RFC3339), maxActivations)
}

func planLabel(tier string) string {
	switch tier {
	case tierPro:
		return "Pro"
	case tierTrial:
		return "Demo"
	default:
		return "Standard"
	}
}

func looksLikeAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "exist") || strings.Contains(s, "duplicate") || strings.Contains(s, "unique")
}
