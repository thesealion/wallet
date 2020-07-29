package wallet

import (
	"context"
	"errors"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/jackc/pgtype"
	shopspring "github.com/jackc/pgtype/ext/shopspring-numeric"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides operations with accounts and payments.
type Service interface {
	// ListAccounts list all the accounts in the system.
	ListAccounts(ctx context.Context) ([]*Account, error)

	// ListPayments list all the accounts in the system.
	ListPayments(ctx context.Context) ([]*Payment, error)

	// SendPayment transfers money between two accounts with the same currency.
	SendPayment(ctx context.Context, fromAccountID, toAccountID string, amount decimal.Decimal) error
}

// Account is the main entity of the wallet service.
type Account struct {
	ID       string          `json:"id"`
	Balance  decimal.Decimal `json:"balance"`
	Currency string          `json:"currency"`
}

// Payment is the entity representing money transfers between accounts.
type Payment struct {
	ID            int
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
}

var (
	// Payment sending errors:

	// ErrAccountNotFound indicates that one or both account IDs are not found.
	ErrAccountNotFound = errors.New("account(s) not found")

	// ErrAccountNotSpecified indicates that one or both account IDs are not specified in the request.
	ErrAccountNotSpecified = errors.New("two accounts must be specified")

	// ErrCurrencyMismatch indicates that accounts have differenct currencies.
	ErrCurrencyMismatch = errors.New("accounts have different currencies")

	// ErrInsufficientBalance indicates that the sender account does not have enough money.
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrInvalidAmount indicates that the requested amount is negative.
	ErrInvalidAmount = errors.New("invalid amount")

	// ErrSameAccount indicates an attempt to make a payment within the same account.
	ErrSameAccount = errors.New("cannot send a payment to the same account")
)

// Service implementation using Postgres for storage.
type service struct {
	db *pgxpool.Pool
}

// NewWalletService creates a new service with Postgres storage.
func NewWalletService(db *pgxpool.Pool) Service {
	return &service{db}
}

// InitDB connects to Postgres using pgx.
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

func (s *service) ListAccounts(ctx context.Context) ([]*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT id, balance, currency FROM accounts ORDER BY id")
	if err != nil {
		return nil, err
	}
	return getAccounts(rows)
}

func (s *service) ListPayments(ctx context.Context) ([]*Payment, error) {
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

func (s *service) SendPayment(ctx context.Context, fromAccountID, toAccountID string, amount decimal.Decimal) error {
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

// Middleware is the service middleware type.
type Middleware func(Service) Service

// LoggingMiddleware is a middleware to log service requests.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{logger, next}
	}
}

type loggingMiddleware struct {
	logger log.Logger
	next   Service
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
