package wallet

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/shopspring/decimal"
)

func makeListAccountsEndpoint(svc WalletService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		_ = request.(listAccountsRequest)
		accounts, err := svc.ListAccounts(ctx)
		return listAccountsResponse{Accounts: accounts, Err: err}, nil
	}
}

func makeListPaymentsEndpoint(svc WalletService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		_ = request.(listPaymentsRequest)
		payments, err := svc.ListPayments(ctx)
		items := make([]*paymentItem, 0, len(payments)*2)
		for _, payment := range payments {
			items = append(items, &paymentItem{
				Account:   payment.FromAccountID,
				Amount:    payment.Amount,
				ToAccount: payment.ToAccountID,
				Direction: "outgoing",
			})
			items = append(items, &paymentItem{
				Account:     payment.ToAccountID,
				Amount:      payment.Amount,
				FromAccount: payment.FromAccountID,
				Direction:   "incoming",
			})
		}
		return listPaymentsResponse{Payments: items, Err: err}, nil
	}
}

func makeSendPaymentEndpoint(svc WalletService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(sendPaymentRequest)
		err := svc.SendPayment(ctx, req.FromAccountID, req.ToAccountID, req.Amount)
		status := "Payment successfully sent"
		if err != nil {
			status = "Payment failed"
		}
		return sendPaymentResponse{Status: status, Err: err}, nil
	}
}

type listAccountsRequest struct{}

type listAccountsResponse struct {
	Accounts []*Account `json:"accounts"`
	Err      error      `json:"err,omitempty"`
}

func (r listAccountsResponse) error() error { return r.Err }

type listPaymentsRequest struct{}

type paymentItem struct {
	Account     string          `json:"account"`
	Amount      decimal.Decimal `json:"amount"`
	FromAccount string          `json:"from_account,omitempty"`
	ToAccount   string          `json:"to_account,omitempty"`
	Direction   string          `json:"direction"`
}

type listPaymentsResponse struct {
	Payments []*paymentItem `json:"payments"`
	Err      error          `json:"err,omitempty"`
}

func (r listPaymentsResponse) error() error { return r.Err }

type sendPaymentRequest struct {
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
}

type sendPaymentResponse struct {
	Status string `json:"status"`
	Err    error  `json:"err,omitempty"`
}

func (r sendPaymentResponse) error() error { return r.Err }
