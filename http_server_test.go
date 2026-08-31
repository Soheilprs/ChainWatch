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
	Page TransferPage
	Err  error

	Query TransferQuery
}

var _ TransferReader = (*MockTransferReader)(nil)

func (m *MockTransferReader) ListTransfers(
	ctx context.Context,
	query TransferQuery,
) (TransferPage, error) {
	m.Query = query

	if m.Err != nil {
		return TransferPage{},
			m.Err
	}

	return m.Page, nil
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
			Page: TransferPage{
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

	var page APITransferPage

	err :=
		json.Unmarshal(
			response.Body.Bytes(),
			&page,
		)

	if err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if len(page.Data) != 1 {
		t.Fatalf(
			"expected 1 transfer, got %d",
			len(page.Data),
		)
	}

	if page.Data[0].Value != "123" {
		t.Fatalf(
			"expected value 123, got %s",
			page.Data[0].Value,
		)
	}

	if page.Pagination.Limit != 100 {
		t.Fatalf(
			"expected default limit 100, got %d",
			page.Pagination.Limit,
		)
	}

	if page.Pagination.HasMore {
		t.Fatal(
			"expected hasMore to be false",
		)
	}

	if page.Pagination.NextCursor != nil {
		t.Fatal(
			"expected nextCursor to be nil",
		)
	}
}

func TestHTTPServerReturnsNextCursor(
	t *testing.T,
) {
	cursor :=
		TransferCursor{
			BlockNumber: 100,
			LogIndex:    20,
		}

	reader :=
		&MockTransferReader{
			Page: TransferPage{
				Transfers: []StoredERC20Transfer{
					{
						BlockNumber: 100,

						BlockHash: "0xblock",

						TransactionHash: "0xtx",

						LogIndex: 20,

						Token: "0xtoken",

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(
							123,
						),
					},
				},

				NextCursor: &cursor,
			},
		}

	server :=
		NewHTTPServer(
			reader,
		)

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers?limit=1",
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

	var page APITransferPage

	err :=
		json.Unmarshal(
			response.Body.Bytes(),
			&page,
		)

	if err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	if !page.Pagination.HasMore {
		t.Fatal(
			"expected hasMore to be true",
		)
	}

	if page.Pagination.NextCursor ==
		nil {

		t.Fatal(
			"expected nextCursor",
		)
	}

	decoded, err :=
		DecodeTransferCursor(
			*page.Pagination.
				NextCursor,
		)

	if err != nil {
		t.Fatalf(
			"failed to decode returned cursor: %v",
			err,
		)
	}

	if decoded.BlockNumber != 100 {
		t.Fatalf(
			"expected cursor block 100, got %d",
			decoded.BlockNumber,
		)
	}

	if decoded.LogIndex != 20 {
		t.Fatalf(
			"expected cursor log index 20, got %d",
			decoded.LogIndex,
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

func TestHTTPServerParsesCursor(
	t *testing.T,
) {
	reader :=
		&MockTransferReader{}

	server :=
		NewHTTPServer(
			reader,
		)

	cursor, err :=
		EncodeTransferCursor(
			TransferCursor{
				BlockNumber: 500,
				LogIndex:    12,
			},
		)

	if err != nil {
		t.Fatalf(
			"failed to encode cursor: %v",
			err,
		)
	}

	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/transfers?cursor="+cursor,
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

	if reader.Query.Cursor == nil {
		t.Fatal(
			"expected cursor",
		)
	}

	if reader.Query.Cursor.
		BlockNumber != 500 {

		t.Fatalf(
			"expected cursor block 500, got %d",
			reader.Query.Cursor.BlockNumber,
		)
	}

	if reader.Query.Cursor.
		LogIndex != 12 {

		t.Fatalf(
			"expected cursor log index 12, got %d",
			reader.Query.Cursor.LogIndex,
		)
	}
}

func TestHTTPServerRejectsInvalidCursor(
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
			"/transfers?cursor=this-is-not-a-valid-cursor",
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

func TestHTTPServerRejectsLimitAboveMaximum(
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
			"/transfers?limit=1001",
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
