package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db   *sql.DB
	path string
}

func Open(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Storage{db: db, path: path}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Storage) Close() error { return s.db.Close() }
func (s *Storage) Path() string { return s.path }

const schema = `
CREATE TABLE IF NOT EXISTS subscriptions (
  telegram_user_id INTEGER PRIMARY KEY,
  username TEXT,
  license_key TEXT NOT NULL,
  tier TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  remind_7d_sent INTEGER NOT NULL DEFAULT 0,
  remind_3d_sent INTEGER NOT NULL DEFAULT 0,
  remind_1d_sent INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS pending_invoices (
  invoice_id INTEGER PRIMARY KEY,
  telegram_user_id INTEGER NOT NULL,
  tier TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  telegram_user_id INTEGER PRIMARY KEY,
  username TEXT,
  inviter_id INTEGER,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trial_claims (
  telegram_user_id INTEGER PRIMARY KEY,
  license_key TEXT NOT NULL,
  claimed_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS promo_codes (
  code TEXT PRIMARY KEY,
  percent_off INTEGER NOT NULL,
  tier TEXT NOT NULL,
  max_uses INTEGER NOT NULL,
  used_count INTEGER NOT NULL DEFAULT 0,
  expires_at TEXT,
  created_at TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS promo_redemptions (
  code TEXT NOT NULL,
  telegram_user_id INTEGER NOT NULL,
  used_at TEXT NOT NULL,
  PRIMARY KEY (code, telegram_user_id)
);

CREATE TABLE IF NOT EXISTS active_promos (
  telegram_user_id INTEGER PRIMARY KEY,
  code TEXT NOT NULL,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS referral_rewards (
  invited_user_id INTEGER PRIMARY KEY,
  inviter_id INTEGER NOT NULL,
  invoice_id INTEGER NOT NULL,
  bonus_days INTEGER NOT NULL,
  rewarded_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  telegram_user_id INTEGER,
  event_type TEXT NOT NULL,
  payload_json TEXT,
  created_at TEXT NOT NULL
);
`

func (s *Storage) migrate() error {
	cols := []struct {
		table string
		name  string
		ddl   string
	}{
		{"pending_invoices", "status", "ALTER TABLE pending_invoices ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'"},
		{"pending_invoices", "fulfilled_at", "ALTER TABLE pending_invoices ADD COLUMN fulfilled_at TEXT"},
		{"pending_invoices", "processed_at", "ALTER TABLE pending_invoices ADD COLUMN processed_at TEXT"},
		{"pending_invoices", "license_key", "ALTER TABLE pending_invoices ADD COLUMN license_key TEXT"},
		{"pending_invoices", "chat_id", "ALTER TABLE pending_invoices ADD COLUMN chat_id INTEGER"},
		{"pending_invoices", "username", "ALTER TABLE pending_invoices ADD COLUMN username TEXT"},
		{"pending_invoices", "amount_usdt", "ALTER TABLE pending_invoices ADD COLUMN amount_usdt TEXT"},
		{"pending_invoices", "final_amount_usdt", "ALTER TABLE pending_invoices ADD COLUMN final_amount_usdt TEXT"},
		{"pending_invoices", "promo_code", "ALTER TABLE pending_invoices ADD COLUMN promo_code TEXT"},
	}
	for _, col := range cols {
		ok, err := s.hasColumn(col.table, col.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := s.db.Exec(col.ddl); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`UPDATE pending_invoices SET status = 'pending' WHERE status IS NULL OR status = ''`)
	return err
}

func (s *Storage) hasColumn(table, name string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, typ string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &colName, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(colName, name) {
			return true, nil
		}
	}
	return false, rows.Err()
}

type Subscription struct {
	TelegramUserID int64
	Username       string
	LicenseKey     string
	Tier           string
	ExpiresAt      time.Time
	Remind7d       bool
	Remind3d       bool
	Remind1d       bool
}

func (s *Storage) GetSubscription(ctx context.Context, telegramUserID int64) (*Subscription, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT telegram_user_id, username, license_key, tier, expires_at, remind_7d_sent, remind_3d_sent, remind_1d_sent
		 FROM subscriptions WHERE telegram_user_id = ?`, telegramUserID)
	var sub Subscription
	var expStr string
	var r7, r3, r1 int
	if err := row.Scan(&sub.TelegramUserID, &sub.Username, &sub.LicenseKey, &sub.Tier, &expStr, &r7, &r3, &r1); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		return nil, err
	}
	sub.ExpiresAt = t.UTC()
	sub.Remind7d = r7 != 0
	sub.Remind3d = r3 != 0
	sub.Remind1d = r1 != 0
	return &sub, nil
}

func (s *Storage) InsertSubscription(ctx context.Context, telegramUserID int64, username, licenseKey, tier string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	exp := expiresAt.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (telegram_user_id, username, license_key, tier, expires_at, created_at, remind_7d_sent, remind_3d_sent, remind_1d_sent)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0)
	`, telegramUserID, username, licenseKey, tier, exp, now)
	return err
}

func (s *Storage) UpdateExpiryAndTier(ctx context.Context, telegramUserID int64, username, tier string, expiresAt time.Time) error {
	exp := expiresAt.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE subscriptions SET expires_at = ?, username = ?, tier = ?, remind_7d_sent = 0, remind_3d_sent = 0, remind_1d_sent = 0
		WHERE telegram_user_id = ?
	`, exp, username, tier, telegramUserID)
	return err
}

func (s *Storage) SetReminderFlags(ctx context.Context, telegramUserID int64, d7, d3, d1 bool) error {
	v7, v3, v1 := 0, 0, 0
	if d7 {
		v7 = 1
	}
	if d3 {
		v3 = 1
	}
	if d1 {
		v1 = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE subscriptions SET remind_7d_sent = ?, remind_3d_sent = ?, remind_1d_sent = ? WHERE telegram_user_id = ?`,
		v7, v3, v1, telegramUserID)
	return err
}

type PendingInvoice struct {
	InvoiceID       int64
	TelegramUserID  int64
	ChatID          int64
	Username        string
	Tier            string
	Status          string
	CreatedAt       string
	FulfilledAt     sql.NullString
	ProcessedAt     sql.NullString
	LicenseKey      sql.NullString
	AmountUSDT      string
	FinalAmountUSDT string
	PromoCode       string
}

func (s *Storage) AddPendingInvoice(ctx context.Context, invoice PendingInvoice) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if invoice.Status == "" {
		invoice.Status = "pending"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO pending_invoices
		  (invoice_id, telegram_user_id, chat_id, username, tier, status, created_at, amount_usdt, final_amount_usdt, promo_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, invoice.InvoiceID, invoice.TelegramUserID, invoice.ChatID, invoice.Username, invoice.Tier, invoice.Status, now, invoice.AmountUSDT, invoice.FinalAmountUSDT, invoice.PromoCode)
	return err
}

func (s *Storage) GetPendingInvoice(ctx context.Context, invoiceID int64) (*PendingInvoice, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT invoice_id, telegram_user_id, COALESCE(chat_id, telegram_user_id), COALESCE(username, ''), tier, status, created_at,
		       fulfilled_at, processed_at, license_key, COALESCE(amount_usdt, ''), COALESCE(final_amount_usdt, ''), COALESCE(promo_code, '')
		FROM pending_invoices WHERE invoice_id = ?
	`, invoiceID)
	var inv PendingInvoice
	if err := row.Scan(&inv.InvoiceID, &inv.TelegramUserID, &inv.ChatID, &inv.Username, &inv.Tier, &inv.Status, &inv.CreatedAt, &inv.FulfilledAt, &inv.ProcessedAt, &inv.LicenseKey, &inv.AmountUSDT, &inv.FinalAmountUSDT, &inv.PromoCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (s *Storage) TryMarkInvoicePaidForFulfillment(ctx context.Context, invoiceID int64) (*PendingInvoice, bool, error) {
	cutoff := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices
		SET status = 'paid', processed_at = ?
		WHERE invoice_id = ?
		  AND status != 'fulfilled'
		  AND fulfilled_at IS NULL
		  AND (processed_at IS NULL OR processed_at < ?)
	`, now, invoiceID, cutoff)
	if err != nil {
		return nil, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		inv, err := s.GetPendingInvoice(ctx, invoiceID)
		return inv, false, err
	}
	inv, err := s.GetPendingInvoice(ctx, invoiceID)
	return inv, true, err
}

func (s *Storage) ClearInvoiceProcessing(ctx context.Context, invoiceID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices SET processed_at = NULL
		WHERE invoice_id = ? AND status = 'paid' AND fulfilled_at IS NULL
	`, invoiceID)
	return err
}

func (s *Storage) MarkInvoiceFulfilled(ctx context.Context, invoiceID int64, licenseKey string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices
		SET status = 'fulfilled', fulfilled_at = ?, processed_at = ?, license_key = ?
		WHERE invoice_id = ?
	`, now, now, licenseKey, invoiceID)
	return err
}

func (s *Storage) SetInvoiceLicenseKey(ctx context.Context, invoiceID int64, licenseKey string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices SET license_key = ?
		WHERE invoice_id = ? AND status != 'fulfilled'
	`, licenseKey, invoiceID)
	return err
}

func (s *Storage) MarkInvoiceExpired(ctx context.Context, invoiceID int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices
		SET status = 'expired', processed_at = ?
		WHERE invoice_id = ? AND status != 'fulfilled'
	`, now, invoiceID)
	return err
}

func (s *Storage) ListInvoicesToResume(ctx context.Context) ([]PendingInvoice, error) {
	cutoff := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT invoice_id, telegram_user_id, COALESCE(chat_id, telegram_user_id), COALESCE(username, ''), tier, status, created_at,
		       fulfilled_at, processed_at, license_key, COALESCE(amount_usdt, ''), COALESCE(final_amount_usdt, ''), COALESCE(promo_code, '')
		FROM pending_invoices
		WHERE status = 'pending'
		   OR (status = 'paid' AND fulfilled_at IS NULL AND (processed_at IS NULL OR processed_at < ?))
		ORDER BY created_at ASC
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingInvoice
	for rows.Next() {
		var inv PendingInvoice
		if err := rows.Scan(&inv.InvoiceID, &inv.TelegramUserID, &inv.ChatID, &inv.Username, &inv.Tier, &inv.Status, &inv.CreatedAt, &inv.FulfilledAt, &inv.ProcessedAt, &inv.LicenseKey, &inv.AmountUSDT, &inv.FinalAmountUSDT, &inv.PromoCode); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Storage) ResetOpenInvoiceProcessing(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE pending_invoices SET processed_at = NULL
		WHERE status = 'paid' AND fulfilled_at IS NULL
	`)
	return err
}

func (s *Storage) CountPendingInvoices(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pending_invoices WHERE status = 'pending'`).Scan(&n)
	return n, err
}

func (s *Storage) CountSubscriptions(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&n)
	return n, err
}

func (s *Storage) DeleteSubscription(ctx context.Context, telegramUserID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE telegram_user_id = ?`, telegramUserID); err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM pending_invoices WHERE telegram_user_id = ?`, telegramUserID)
	return nil
}

type PendingInvoiceRow struct {
	InvoiceID      int64
	TelegramUserID int64
	Tier           string
	Status         string
	CreatedAt      string
}

func (s *Storage) ListPendingInvoices(ctx context.Context, limit int) ([]PendingInvoiceRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT invoice_id, telegram_user_id, tier, status, created_at FROM pending_invoices WHERE status IN ('pending', 'paid') ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingInvoiceRow
	for rows.Next() {
		var r PendingInvoiceRow
		if err := rows.Scan(&r.InvoiceID, &r.TelegramUserID, &r.Tier, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Storage) TelegramIDByLicenseKey(ctx context.Context, licenseKey string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT telegram_user_id FROM subscriptions WHERE license_key = ?`, licenseKey).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (s *Storage) ListSubscriptionsExpiringWithin(ctx context.Context, maxDays int) ([]Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT telegram_user_id, username, license_key, tier, expires_at, remind_7d_sent, remind_3d_sent, remind_1d_sent
		FROM subscriptions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, maxDays)
	for rows.Next() {
		var sub Subscription
		var expStr string
		var r7, r3, r1 int
		if err := rows.Scan(&sub.TelegramUserID, &sub.Username, &sub.LicenseKey, &sub.Tier, &expStr, &r7, &r3, &r1); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, expStr)
		if err != nil {
			continue
		}
		sub.ExpiresAt = t.UTC()
		if !sub.ExpiresAt.After(now) || sub.ExpiresAt.After(cutoff) {
			continue
		}
		sub.Remind7d = r7 != 0
		sub.Remind3d = r3 != 0
		sub.Remind1d = r1 != 0
		out = append(out, sub)
	}
	return out, rows.Err()
}

func FormatDaysLeft(exp time.Time) int {
	d := time.Until(exp)
	if d <= 0 {
		return 0
	}
	return int(d.Hours() / 24)
}

func (s *Storage) UpsertUser(ctx context.Context, telegramUserID int64, username string, inviterID *int64) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, username, inviter_id, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(telegram_user_id) DO UPDATE SET
		  username = excluded.username,
		  inviter_id = CASE
		    WHEN users.inviter_id IS NULL AND excluded.inviter_id IS NOT NULL THEN excluded.inviter_id
		    ELSE users.inviter_id
		  END
	`, telegramUserID, username, inviterID, now)
	if err != nil {
		return false, err
	}
	if inviterID == nil {
		return false, nil
	}
	var got sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT inviter_id FROM users WHERE telegram_user_id = ?`, telegramUserID).Scan(&got); err != nil {
		return false, err
	}
	return got.Valid && got.Int64 == *inviterID, nil
}

func (s *Storage) GetInviterID(ctx context.Context, telegramUserID int64) (int64, bool, error) {
	var inviter sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT inviter_id FROM users WHERE telegram_user_id = ?`, telegramUserID).Scan(&inviter)
	if errors.Is(err, sql.ErrNoRows) || !inviter.Valid {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return inviter.Int64, true, nil
}

func (s *Storage) ListAllUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT telegram_user_id FROM users
		UNION
		SELECT telegram_user_id FROM subscriptions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (s *Storage) ListSubscriberIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT telegram_user_id FROM subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func scanIDs(rows *sql.Rows) ([]int64, error) {
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Storage) ReferralRewardExists(ctx context.Context, invitedUserID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM referral_rewards WHERE invited_user_id = ?`, invitedUserID).Scan(&n)
	return n > 0, err
}

func (s *Storage) InsertReferralReward(ctx context.Context, inviterID, invitedUserID, invoiceID int64, bonusDays int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO referral_rewards (inviter_id, invited_user_id, invoice_id, bonus_days, rewarded_at)
		VALUES (?, ?, ?, ?, ?)
	`, inviterID, invitedUserID, invoiceID, bonusDays, now)
	return err
}

func (s *Storage) HasTrialClaim(ctx context.Context, telegramUserID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM trial_claims WHERE telegram_user_id = ?`, telegramUserID).Scan(&n)
	return n > 0, err
}

func (s *Storage) InsertTrialClaim(ctx context.Context, telegramUserID int64, licenseKey string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trial_claims (telegram_user_id, license_key, claimed_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, telegramUserID, licenseKey, now, expiresAt.UTC().Format(time.RFC3339))
	return err
}

type PromoCode struct {
	Code       string
	PercentOff int
	Tier       string
	MaxUses    int
	UsedCount  int
	ExpiresAt  sql.NullString
	CreatedAt  string
	Active     bool
}

func NormalizePromoCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func (s *Storage) CreatePromoCode(ctx context.Context, code string, percentOff int, tier string, maxUses int, expiresAt *time.Time) error {
	code = NormalizePromoCode(code)
	now := time.Now().UTC().Format(time.RFC3339)
	var exp interface{}
	if expiresAt != nil {
		exp = expiresAt.UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO promo_codes (code, percent_off, tier, max_uses, used_count, expires_at, created_at, active)
		VALUES (?, ?, ?, ?, 0, ?, ?, 1)
		ON CONFLICT(code) DO UPDATE SET
		  percent_off = excluded.percent_off,
		  tier = excluded.tier,
		  max_uses = excluded.max_uses,
		  expires_at = excluded.expires_at,
		  active = 1
	`, code, percentOff, tier, maxUses, exp, now)
	return err
}

func (s *Storage) DisablePromoCode(ctx context.Context, code string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE promo_codes SET active = 0 WHERE code = ?`, NormalizePromoCode(code))
	return err
}

func (s *Storage) ListPromoCodes(ctx context.Context, limit int) ([]PromoCode, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT code, percent_off, tier, max_uses, used_count, expires_at, created_at, active
		FROM promo_codes ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PromoCode
	for rows.Next() {
		var p PromoCode
		var active int
		if err := rows.Scan(&p.Code, &p.PercentOff, &p.Tier, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &active); err != nil {
			return nil, err
		}
		p.Active = active != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Storage) GetPromoCode(ctx context.Context, code string) (*PromoCode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT code, percent_off, tier, max_uses, used_count, expires_at, created_at, active
		FROM promo_codes WHERE code = ?
	`, NormalizePromoCode(code))
	var p PromoCode
	var active int
	if err := row.Scan(&p.Code, &p.PercentOff, &p.Tier, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedAt, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.Active = active != 0
	return &p, nil
}

func (s *Storage) SetActivePromo(ctx context.Context, telegramUserID int64, code string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO active_promos (telegram_user_id, code, applied_at)
		VALUES (?, ?, ?)
		ON CONFLICT(telegram_user_id) DO UPDATE SET code = excluded.code, applied_at = excluded.applied_at
	`, telegramUserID, NormalizePromoCode(code), now)
	return err
}

func (s *Storage) GetActivePromo(ctx context.Context, telegramUserID int64) (string, bool, error) {
	var code string
	err := s.db.QueryRowContext(ctx, `SELECT code FROM active_promos WHERE telegram_user_id = ?`, telegramUserID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return code, err == nil, err
}

func (s *Storage) ClearActivePromo(ctx context.Context, telegramUserID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM active_promos WHERE telegram_user_id = ?`, telegramUserID)
	return err
}

func (s *Storage) HasPromoRedemption(ctx context.Context, code string, telegramUserID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM promo_redemptions WHERE code = ? AND telegram_user_id = ?`, NormalizePromoCode(code), telegramUserID).Scan(&n)
	return n > 0, err
}

func (s *Storage) RedeemPromo(ctx context.Context, code string, telegramUserID int64) error {
	code = NormalizePromoCode(code)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `INSERT INTO promo_redemptions (code, telegram_user_id, used_at) VALUES (?, ?, ?)`, code, telegramUserID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE promo_codes SET used_count = used_count + 1 WHERE code = ?`, code); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM active_promos WHERE telegram_user_id = ?`, telegramUserID); err != nil {
		return err
	}
	return tx.Commit()
}

func (p PromoCode) IsValidFor(tier string) (bool, string) {
	if !p.Active {
		return false, "Promo code is disabled."
	}
	if p.UsedCount >= p.MaxUses {
		return false, "Promo code usage limit reached."
	}
	if p.Tier != "any" && p.Tier != tier {
		return false, "Promo code does not match this plan."
	}
	if p.ExpiresAt.Valid {
		exp, err := time.Parse(time.RFC3339, p.ExpiresAt.String)
		if err == nil && !exp.After(time.Now().UTC()) {
			return false, "Promo code has expired."
		}
	}
	return true, ""
}

func (s *Storage) RecordEvent(ctx context.Context, telegramUserID *int64, eventType, payloadJSON string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var id interface{}
	if telegramUserID != nil {
		id = *telegramUserID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (telegram_user_id, event_type, payload_json, created_at) VALUES (?, ?, ?, ?)`, id, eventType, payloadJSON, now)
	return err
}

type Stats struct {
	UsersTotal          int
	SubscriptionsTotal  int
	ActiveSubscriptions int
	PendingInvoices     int
	PaidInvoices        int
	ExpiredInvoices     int
	TrialsClaimed       int
	EventsLast24h       int
	RevenueByTier       map[string]float64
}

func (s *Storage) Stats(ctx context.Context) (*Stats, error) {
	out := &Stats{RevenueByTier: map[string]float64{}}
	now := time.Now().UTC().Format(time.RFC3339)
	since := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	queries := []struct {
		dest *int
		sql  string
		args []interface{}
	}{
		{&out.UsersTotal, `SELECT COUNT(*) FROM users`, nil},
		{&out.SubscriptionsTotal, `SELECT COUNT(*) FROM subscriptions`, nil},
		{&out.ActiveSubscriptions, `SELECT COUNT(*) FROM subscriptions WHERE expires_at > ?`, []interface{}{now}},
		{&out.PendingInvoices, `SELECT COUNT(*) FROM pending_invoices WHERE status = 'pending'`, nil},
		{&out.PaidInvoices, `SELECT COUNT(*) FROM pending_invoices WHERE status IN ('paid', 'fulfilled')`, nil},
		{&out.ExpiredInvoices, `SELECT COUNT(*) FROM pending_invoices WHERE status = 'expired'`, nil},
		{&out.TrialsClaimed, `SELECT COUNT(*) FROM trial_claims`, nil},
		{&out.EventsLast24h, `SELECT COUNT(*) FROM events WHERE created_at >= ?`, []interface{}{since}},
	}
	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dest); err != nil {
			return nil, err
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tier, COALESCE(final_amount_usdt, amount_usdt, '0')
		FROM pending_invoices WHERE status = 'fulfilled'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tier, amount string
		if err := rows.Scan(&tier, &amount); err != nil {
			return nil, err
		}
		v, _ := strconv.ParseFloat(amount, 64)
		out.RevenueByTier[tier] += v
	}
	return out, rows.Err()
}

func MaskLicenseKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	parts := strings.Split(key, "-")
	if len(parts) == 4 {
		return parts[0] + "-****-****-" + parts[3]
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "-****-" + key[len(key)-4:]
}

func FormatUSDT(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
