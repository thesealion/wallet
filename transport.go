package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
)

var (
	// ErrMalformedJSON is an error of JSON decoding.
	ErrMalformedJSON = errors.New("unable to parse request")
)

// MakeHTTPHandler returns an HTTP handler for Service.
func MakeHTTPHandler(svc Service, logger log.Logger) http.Handler {
	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(transport.NewLogErrorHandler(logger)),
		httptransport.ServerErrorEncoder(encodeError),
	}

	// GET   /accounts  list accounts
	// GET   /payments  list payments
	// POST  /payments  send a payment

	r.Methods("GET").Path("/accounts").Handler(httptransport.NewServer(
		makeListAccountsEndpoint(svc),
		decodeListAccountsRequest,
		encodeResponse,
		options...,
	))
	r.Methods("GET").Path("/payments").Handler(httptransport.NewServer(
		makeListPaymentsEndpoint(svc),
		decodeListPaymentsRequest,
		encodeResponse,
		options...,
	))
	r.Methods("POST").Path("/payments").Handler(httptransport.NewServer(
		makeSendPaymentEndpoint(svc),
		decodeSendPaymentRequest,
		encodeResponse,
		options...,
	))
	return r
}

func decodeListAccountsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return listAccountsRequest{}, nil
}

func decodeListPaymentsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return listPaymentsRequest{}, nil
}

func decodeSendPaymentRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req sendPaymentRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(&req); e != nil {
		return nil, ErrMalformedJSON
	}
	return req, nil
}

type errorer interface {
	error() error
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if e, ok := response.(errorer); ok && e.error() != nil {
		// Business-logic error.
		encodeError(ctx, e.error(), w)
		return nil
	}
	return json.NewEncoder(w).Encode(response)
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	code := codeFrom(err)
	msg := err.Error()
	if code == http.StatusInternalServerError {
		msg = "internal server error"
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": msg,
	})
}

func codeFrom(err error) int {
	switch err {
	case ErrCurrencyMismatch, ErrInsufficientBalance, ErrInvalidAmount, ErrSameAccount:
		return http.StatusForbidden
	case ErrAccountNotFound:
		return http.StatusNotFound
	case ErrMalformedJSON, ErrAccountNotSpecified:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
