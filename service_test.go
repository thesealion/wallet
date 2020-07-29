package wallet

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

func TestListAccounts(t *testing.T) {
	dbpool, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()
	svc := NewWalletService(dbpool)

	accounts, err := svc.ListAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 3 {
		t.Fatal("wrong number of accounts")
	}
}

func TestListPayments(t *testing.T) {
	dbpool, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()
	svc := NewWalletService(dbpool)
	ctx := context.Background()

	payments, err := svc.ListPayments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 0 {
		t.Fatal("wrong number of payments")
	}

	err = svc.SendPayment(ctx, "bob123", "alice456", decimal.NewFromInt(10))
	if err != nil {
		t.Fatal(err)
	}
	err = svc.SendPayment(ctx, "bob123", "alice456", decimal.RequireFromString("0.5"))
	if err != nil {
		t.Fatal(err)
	}

	payments, err = svc.ListPayments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 2 {
		t.Fatal("payments not created")
	}
	if payments[0].FromAccountID != payments[1].FromAccountID || payments[0].FromAccountID != "bob123" {
		t.Error("wrong fromAccountID")
	}
	if payments[0].ToAccountID != payments[1].ToAccountID || payments[0].ToAccountID != "alice456" {
		t.Error("wrong toAccountID")
	}
	if !payments[0].Amount.Equal(decimal.NewFromInt(10)) || !payments[1].Amount.Equal(decimal.RequireFromString("0.5")) {
		t.Error("wrong amount")
	}
}

func TestSendPayment(t *testing.T) {
	dbpool, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	defer dbpool.Close()
	svc := NewWalletService(dbpool)
	ctx := context.Background()

	// Bad IDs
	err = svc.SendPayment(ctx, "", "", decimal.NewFromInt(1))
	if err != ErrAccountNotSpecified {
		t.Errorf("%v instead of ErrAccountNotSpecified", err)
	}
	err = svc.SendPayment(ctx, "testid", "", decimal.NewFromInt(1))
	if err != ErrAccountNotSpecified {
		t.Errorf("%v instead of ErrAccountNotSpecified", err)
	}
	err = svc.SendPayment(ctx, "testid", "testid", decimal.NewFromInt(1))
	if err != ErrSameAccount {
		t.Errorf("%v instead of ErrSameAccount", err)
	}

	// Bad amount
	err = svc.SendPayment(ctx, "testid1", "testid2", decimal.NewFromInt(-1))
	if err != ErrInvalidAmount {
		t.Errorf("%v instead of ErrInvalidAmount", err)
	}

	// Non-existing accounts
	err = svc.SendPayment(ctx, "testid1", "testid2", decimal.NewFromInt(1))
	if err != ErrAccountNotFound {
		t.Errorf("%v instead of ErrAccountNotFound", err)
	}

	// Different currencies
	err = svc.SendPayment(ctx, "alice456", "eve789", decimal.NewFromInt(1))
	if err != ErrCurrencyMismatch {
		t.Errorf("%v instead of ErrCurrencyMismatch", err)
	}

	// Not enough money
	err = svc.SendPayment(ctx, "alice456", "bob123", decimal.NewFromInt(100))
	if err != ErrInsufficientBalance {
		t.Errorf("%v instead of ErrInsufficientBalance", err)
	}

	// OK
	err = svc.SendPayment(ctx, "bob123", "alice456", decimal.NewFromInt(10))
	if err != nil {
		t.Fatal(err)
	}
}
