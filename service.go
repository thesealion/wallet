package wallet

import (
	"context"
	"errors"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jackc/pgtype"
	shopspring "github.com/jackc/pgtype/ext/shopspring-numeric"
	"github.com/shopspring/decimal"
)

// WalletService provides operations with accounts and payments.
type WalletService interface {
	// ListAccounts list all the accounts in the system.
	ListAccounts(ctx context.Context) ([]*Account, error)

	// ListPayments list all the accounts in the system.
	ListPayments(ctx context.Context) ([]*Payment, error)

	// SendPayment transfers money between two accounts with the same currency.
	SendPayment(ctx context.Context, fromAccountID, toAccountID string, amount decimal.Decimal) error
}

type Account struct {
	ID       string          `json:"id"`
	Balance  decimal.Decimal `json:"balance"`
	Currency string          `json:"currency"`
}

type Payment struct {
	ID            int
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
}

var (
	ErrAccountNotFound     = errors.New("account(s) not found")
	ErrAccountNotSpecified = errors.New("two accounts must be specified")
	ErrCurrencyMismatch    = errors.New("accounts have different currencies")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrSameAccount         = errors.New("cannot send a payment to the same account")
)

// WalletService implementation using Postgres for storage.
type walletService struct {
	db *pgxpool.Pool
}

func NewWalletService(db *pgxpool.Pool) WalletService {
	return &walletService{db}
}

// Connect to Postgres using pgx.
func InitDB() (*pgxpool.Pool, error) {
	// Register decimal data type with pgx
	config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.ConnInfo().RegisterDataType(pgtype.DataType{
			Value: &shopspring.Numeric{},
			Name:  "numeric",
			OID:   pgtype.NumericOID,
		})
		return nil
	}
	dbpool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
        return nil, err
	}
    return dbpool, nil
}

// Read accounts from DB into a slice
func getAccounts(rows pgx.Rows) ([]*Account, error) {
	accounts := make([]*Account, 0)
	for rows.Next() {
		var (
			id       string
			balance  decimal.Decimal
			currency string
		)
		err := rows.Scan(&id, &balance, &currency)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, &Account{id, balance, currency})
	}
	return accounts, nil
}

func (s *walletService) ListAccounts(ctx context.Context) ([]*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT id, balance, currency FROM accounts ORDER BY id")
	if err != nil {
		return nil, err
	}
	return getAccounts(rows)
}

func (s *walletService) ListPayments(ctx context.Context) ([]*Payment, error) {
	payments := make([]*Payment, 0)
	rows, err := s.db.Query(ctx, "SELECT id, from_account_id, to_account_id, amount FROM payments ORDER BY id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id            int
			fromAccountID string
			toAccountID   string
			amount        decimal.Decimal
		)
		err = rows.Scan(&id, &fromAccountID, &toAccountID, &amount)
		if err != nil {
			return nil, err
		}
		payments = append(payments, &Payment{id, fromAccountID, toAccountID, amount})
	}

	return payments, nil
}

func (s *walletService) SendPayment(ctx context.Context, fromAccountID, toAccountID string, amount decimal.Decimal) error {
	// Check parameters
	if fromAccountID == "" || toAccountID == "" {
		return ErrAccountNotSpecified
	}
	if fromAccountID == toAccountID {
		return ErrSameAccount
	}
	if !amount.IsPositive() {
		return ErrInvalidAmount
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT id, balance, currency FROM accounts WHERE id = $1 OR id = $2 ORDER BY id FOR UPDATE", fromAccountID, toAccountID)
	if err != nil {
		return err
	}
	accounts, err := getAccounts(rows)
	if err != nil {
		return err
	}
	accountsByID := make(map[string]*Account, 2)
	for _, account := range accounts {
		accountsByID[account.ID] = account
	}
	fromAccount := accountsByID[fromAccountID]
	toAccount := accountsByID[toAccountID]

	if fromAccount == nil || toAccount == nil {
		return ErrAccountNotFound
	}
	if fromAccount.Currency != toAccount.Currency {
		return ErrCurrencyMismatch
	}
	fromAccount.Balance = fromAccount.Balance.Sub(amount)
	toAccount.Balance = toAccount.Balance.Add(amount)
	if fromAccount.Balance.IsNegative() {
		return ErrInsufficientBalance
	}

	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", fromAccount.Balance, fromAccount.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", toAccount.Balance, toAccount.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO payments (from_account_id, to_account_id, amount) VALUES ($1, $2, $3)", fromAccount.ID, toAccount.ID, amount)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Service middleware type.
type Middleware func(WalletService) WalletService

// Middleware to log service requests.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next WalletService) WalletService {
		return loggingMiddleware{logger, next}
	}
}

type loggingMiddleware struct {
	logger log.Logger
	next   WalletService
}

func (mw loggingMiddleware) ListAccounts(ctx context.Context) (accounts []*Account, err error) {
	defer func() {
		mw.logger.Log("method", "ListAccounts", "err", err)
	}()
	accounts, err = mw.next.ListAccounts(ctx)
	return
}

func (mw loggingMiddleware) ListPayments(ctx context.Context) (payments []*Payment, err error) {
	defer func() {
		mw.logger.Log("method", "ListPayments", "err", err)
	}()
	payments, err = mw.next.ListPayments(ctx)
	return
}

func (mw loggingMiddleware) SendPayment(ctx context.Context, fromAccountID, toAccountID string, amount decimal.Decimal) (err error) {
	defer func() {
		mw.logger.Log("method", "SendPayment", "fromAccountID", fromAccountID, "toAccountID", toAccountID, "amount", amount, "err", err)
	}()
	err = mw.next.SendPayment(ctx, fromAccountID, toAccountID, amount)
	return
}
