package main

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockTransferReader struct {
	Transfers []StoredERC20Transfer
	Err       error

	Query TransferQuery
}

var _ TransferReader = (*MockTransferReader)(nil)

func (m *MockTransferReader) ListTransfers(
	ctx context.Context,
	query TransferQuery,
) ([]StoredERC20Transfer, error) {
	m.Query = query

	if m.Err != nil {
		return nil, m.Err
	}

	return m.Transfers, nil
}

func TestHTTPServerHealth(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/health",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusOK {

		t.Fatalf(
			"expected status 200, got %d",
			response.Code,
		)
	}
}

func TestHTTPServerListTransfers(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{
			Transfers: []StoredERC20Transfer{
				{
					BlockNumber: 100,

					BlockHash: "0xblock",

					TransactionHash: "0xtx",

					LogIndex: 2,

					Token: "0xtoken",

					From: "0xfrom",

					To: "0xto",

					Value: big.NewInt(
						123,
					),
				},
			},
		}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusOK {

		t.Fatalf(
			"expected status 200, got %d",
			response.Code,
		)
	}

	var transfers []APITransfer

	err :=
		json.Unmarshal(
			response.Body.Bytes(),
			&transfers,
		)

	if err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if len(transfers) != 1 {
		t.Fatalf(
			"expected 1 transfer, got %d",
			len(transfers),
		)
	}

	if transfers[0].Value != "123" {
		t.Fatalf(
			"expected value 123, got %s",
			transfers[0].Value,
		)
	}
}

func TestHTTPServerParsesTransferFilters(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers?block=123&token=0xtoken&address=0xwallet&limit=25",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusOK {

		t.Fatalf(
			"expected status 200, got %d",
			response.Code,
		)
	}

	if reader.Query.BlockNumber ==
		nil {

		t.Fatal(
			"expected block filter",
		)
	}

	if *reader.Query.BlockNumber !=
		123 {

		t.Fatalf(
			"expected block 123, got %d",
			*reader.Query.BlockNumber,
		)
	}

	if reader.Query.Token ==
		nil {

		t.Fatal(
			"expected token filter",
		)
	}

	if *reader.Query.Token !=
		"0xtoken" {

		t.Fatalf(
			"expected token 0xtoken, got %s",
			*reader.Query.Token,
		)
	}

	if reader.Query.Address ==
		nil {

		t.Fatal(
			"expected address filter",
		)
	}

	if *reader.Query.Address !=
		"0xwallet" {

		t.Fatalf(
			"expected address 0xwallet, got %s",
			*reader.Query.Address,
		)
	}

	if reader.Query.Limit != 25 {
		t.Fatalf(
			"expected limit 25, got %d",
			reader.Query.Limit,
		)
	}
}

func TestHTTPServerRejectsInvalidBlock(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers?block=abc",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected status 400, got %d",
			response.Code,
		)
	}
}

func TestHTTPServerRejectsInvalidLimit(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers?limit=-5",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusBadRequest {

		t.Fatalf(
			"expected status 400, got %d",
			response.Code,
		)
	}
}

func TestHTTPServerRejectsPostTransfers(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/transfers",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusMethodNotAllowed {

		t.Fatalf(
			"expected status 405, got %d",
			response.Code,
		)
	}
}

func TestHTTPServerHandlesReaderError(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{
			Err: errors.New(
				"database exploded",
			),
		}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers",
			nil,
		)

	response :=
		httptest.NewRecorder()

	server.Handler().
		ServeHTTP(
			response,
			request,
		)

	if response.Code !=
		http.StatusInternalServerError {

		t.Fatalf(
			"expected status 500, got %d",
			response.Code,
		)
	}
}
