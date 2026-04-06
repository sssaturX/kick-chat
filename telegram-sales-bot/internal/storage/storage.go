package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

func Open(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Storage{db: db}, nil
}

func (s *Storage) Close() error { return s.db.Close() }

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
`

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

func (s *Storage) AddPendingInvoice(ctx context.Context, invoiceID, telegramUserID int64, tier string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO pending_invoices (invoice_id, telegram_user_id, tier, created_at) VALUES (?, ?, ?, ?)`,
		invoiceID, telegramUserID, tier, now)
	return err
}

func (s *Storage) RemovePendingInvoice(ctx context.Context, invoiceID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pending_invoices WHERE invoice_id = ?`, invoiceID)
	return err
}

// ListSubscriptionsExpiringWithin returns subs still active that expire within maxDays (for reminder scan).
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

func (s *Storage) DeleteSubscription(ctx context.Context, telegramUserID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE telegram_user_id = ?`, telegramUserID); err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM pending_invoices WHERE telegram_user_id = ?`, telegramUserID)
	return nil
}

func (s *Storage) CountSubscriptions(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&n)
	return n, err
}

func (s *Storage) CountPendingInvoices(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pending_invoices`).Scan(&n)
	return n, err
}

type PendingInvoiceRow struct {
	InvoiceID      int64
	TelegramUserID int64
	Tier           string
	CreatedAt      string
}

func (s *Storage) ListPendingInvoices(ctx context.Context, limit int) ([]PendingInvoiceRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT invoice_id, telegram_user_id, tier, created_at FROM pending_invoices ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingInvoiceRow
	for rows.Next() {
		var r PendingInvoiceRow
		if err := rows.Scan(&r.InvoiceID, &r.TelegramUserID, &r.Tier, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TelegramIDByLicenseKey returns subscriber id for a license key, if known to the bot.
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
