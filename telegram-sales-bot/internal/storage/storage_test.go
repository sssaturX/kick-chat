package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStorage(t *testing.T) *Storage {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestInvoiceFulfillmentIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := openTestStorage(t)

	if err := st.AddPendingInvoice(ctx, PendingInvoice{
		InvoiceID:       101,
		TelegramUserID:  777,
		ChatID:          777,
		Tier:            "standard",
		AmountUSDT:      "29",
		FinalAmountUSDT: "29",
	}); err != nil {
		t.Fatal(err)
	}

	inv, claimed, err := st.TryMarkInvoicePaidForFulfillment(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed || inv == nil || inv.Status != "paid" {
		t.Fatalf("expected first claim to win, got claimed=%v inv=%+v", claimed, inv)
	}
	if _, claimed, err := st.TryMarkInvoicePaidForFulfillment(ctx, 101); err != nil {
		t.Fatal(err)
	} else if claimed {
		t.Fatal("second claim should not win while processing lock is set")
	}
	if err := st.MarkInvoiceFulfilled(ctx, 101, "ABCD-EFGH-JKLM-2345"); err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := st.TryMarkInvoicePaidForFulfillment(ctx, 101); err != nil {
		t.Fatal(err)
	} else if claimed {
		t.Fatal("fulfilled invoice must not be claimed again")
	}
}

func TestTrialPromoAndReferralTables(t *testing.T) {
	ctx := context.Background()
	st := openTestStorage(t)

	exp := time.Now().UTC().Add(24 * time.Hour)
	if err := st.InsertTrialClaim(ctx, 1, "TRIA-LKEY-TEST-0001", exp); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.HasTrialClaim(ctx, 1); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("trial claim should be stored")
	}

	promoExp := time.Now().UTC().AddDate(0, 0, 7)
	if err := st.CreatePromoCode(ctx, "SAVE20", 20, "any", 10, &promoExp); err != nil {
		t.Fatal(err)
	}
	promo, err := st.GetPromoCode(ctx, "save20")
	if err != nil {
		t.Fatal(err)
	}
	if promo == nil || promo.PercentOff != 20 {
		t.Fatalf("promo not loaded correctly: %+v", promo)
	}
	if err := st.SetActivePromo(ctx, 1, "save20"); err != nil {
		t.Fatal(err)
	}
	if err := st.RedeemPromo(ctx, "SAVE20", 1); err != nil {
		t.Fatal(err)
	}
	if used, err := st.HasPromoRedemption(ctx, "SAVE20", 1); err != nil {
		t.Fatal(err)
	} else if !used {
		t.Fatal("promo redemption should be stored")
	}

	inviter := int64(10)
	registered, err := st.UpsertUser(ctx, 20, "buyer", &inviter)
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("referral should be registered")
	}
	got, ok, err := st.GetInviterID(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != inviter {
		t.Fatalf("inviter mismatch: got %d ok=%v", got, ok)
	}
	if err := st.InsertReferralReward(ctx, inviter, 20, 101, 7); err != nil {
		t.Fatal(err)
	}
	if exists, err := st.ReferralRewardExists(ctx, 20); err != nil {
		t.Fatal(err)
	} else if !exists {
		t.Fatal("referral reward should exist")
	}
}
