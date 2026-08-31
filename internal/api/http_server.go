package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/soheilprs/chainwatch/internal/domain"
	"github.com/soheilprs/chainwatch/internal/observability"
)

type HTTPServer struct {
	transferReader        TransferReader
	tokenMetadataProvider TokenMetadataProvider

	logger  *slog.Logger
	metrics *observability.Metrics
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

type APIErrorResponse struct {
	Error APIError `json:"error"`
}

func NewHTTPServer(
	transferReader TransferReader,
	tokenMetadataProvider TokenMetadataProvider,
) *HTTPServer {
	return NewHTTPServerWithObservability(
		transferReader,
		tokenMetadataProvider,
		slog.New(
			slog.NewTextHandler(
				io.Discard,
				nil,
			),
		),
		observability.NewMetrics(),
	)
}

func NewHTTPServerWithObservability(
	transferReader TransferReader,
	tokenMetadataProvider TokenMetadataProvider,
	logger *slog.Logger,
	metrics *observability.Metrics,
) *HTTPServer {
	if logger == nil {
		logger =
			slog.New(
				slog.NewTextHandler(
					io.Discard,
					nil,
				),
			)
	}

	if metrics == nil {
		metrics =
			observability.NewMetrics()
	}

	return &HTTPServer{
		transferReader: transferReader,

		tokenMetadataProvider: tokenMetadataProvider,

		logger: logger,

		metrics: metrics,
	}
}

func (s *HTTPServer) Handler() http.Handler {
	mux :=
		http.NewServeMux()

	mux.HandleFunc(
		"/health",
		s.handleHealth,
	)

	mux.HandleFunc(
		"/transfers",
		s.handleTransfers,
	)

	mux.HandleFunc(
		"/metrics",
		s.handleMetrics,
	)

	return requestIDMiddleware(
		observabilityMiddleware(
			s.logger,
			s.metrics,
			recoveryMiddleware(s.logger, mux),
		),
	)
}

func (s *HTTPServer) handleHealth(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, r)
		return
	}

	s.writeJSONResponse(
		r,
		w,
		http.StatusOK,
		map[string]string{
			"status": "ok",
		},
	)
}

func (s *HTTPServer) handleMetrics(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, r)
		return
	}

	w.Header().Set(
		"Content-Type",
		"text/plain; version=0.0.4; charset=utf-8",
	)

	if err :=
		s.metrics.WritePrometheus(w); err != nil {

		s.logger.ErrorContext(
			r.Context(),
			"failed to write metrics",
			"error",
			err,
		)
	}
}

func (s *HTTPServer) handleTransfers(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, r)
		return
	}

	query, err :=
		parseTransferQuery(r)

	if err != nil {
		s.writeHTTPError(w, r, err)
		return
	}

	page, err :=
		s.transferReader.ListTransfers(
			r.Context(),
			query,
		)

	if err != nil {
		s.writeHTTPError(w, r, err)
		return
	}

	data := make(
		[]APITransfer,
		0,
		len(page.Transfers),
	)

	for _, transfer := range page.Transfers {

		apiTransfer :=
			apiTransferFromStored(
				transfer,
			)

		if s.tokenMetadataProvider != nil {
			metadata, err :=
				s.tokenMetadataProvider.
					GetTokenMetadata(
						r.Context(),
						transfer.Token,
					)

			if err != nil {
				s.metrics.
					RecordTokenMetadataError()

				s.logger.WarnContext(
					r.Context(),
					"token metadata unavailable",
					"token",
					string(
						transfer.Token,
					),
					"error",
					err,
				)
			} else {
				enrichAPITransferWithMetadata(
					&apiTransfer,
					transfer,
					metadata,
				)
			}
		}

		data = append(
			data,
			apiTransfer,
		)
	}

	var nextCursor *string

	if page.NextCursor != nil {
		encodedCursor, err :=
			EncodeTransferCursor(
				*page.NextCursor,
			)

		if err != nil {
			s.logger.ErrorContext(
				r.Context(),
				"failed to encode pagination cursor",
				"error",
				err,
			)

			s.writeHTTPError(w, r, err)
			return
		}

		nextCursor =
			&encodedCursor
	}

	response :=
		APITransferPage{
			Data: data,

			Pagination: APIPagination{
				Limit: query.Limit,

				HasMore: page.NextCursor != nil,

				NextCursor: nextCursor,
			},
		}

	s.writeJSONResponse(
		r,
		w,
		http.StatusOK,
		response,
	)
}

func parseTransferQuery(
	r *http.Request,
) (domain.TransferQuery, error) {
	values :=
		r.URL.Query()

	query :=
		domain.TransferQuery{
			Limit: 100,
		}

	if rawBlock :=
		values.Get("block"); rawBlock != "" {

		blockNumber, err :=
			strconv.ParseUint(
				rawBlock,
				10,
				64,
			)

		if err != nil {
			return domain.TransferQuery{}, NewBadInputError("invalid block", err)
		}

		query.BlockNumber =
			&blockNumber
	}

	if rawToken :=
		values.Get("token"); rawToken != "" {
		if !common.IsHexAddress(rawToken) {
			return domain.TransferQuery{}, NewBadInputError("invalid token address", domain.ErrInvalidTokenAddress)
		}

		token :=
			domain.Address(common.HexToAddress(rawToken).Hex())

		query.Token =
			&token
	}

	if rawAddress :=
		values.Get("address"); rawAddress != "" {
		if !common.IsHexAddress(rawAddress) {
			return domain.TransferQuery{}, NewBadInputError("invalid transfer address", domain.ErrInvalidTokenAddress)
		}

		address :=
			domain.Address(common.HexToAddress(rawAddress).Hex())

		query.Address =
			&address
	}

	if rawLimit :=
		values.Get("limit"); rawLimit != "" {

		limit, err :=
			strconv.Atoi(
				rawLimit,
			)

		if err != nil ||
			limit <= 0 {

			return domain.TransferQuery{}, NewBadInputError("invalid limit", errors.New("limit must be a positive integer"))
		}

		if limit > 1000 {
			return domain.TransferQuery{}, NewBadInputError("limit cannot exceed 1000", errors.New("limit exceeds maximum"))
		}

		query.Limit =
			limit
	}

	if rawCursor :=
		values.Get("cursor"); rawCursor != "" {

		cursor, err :=
			DecodeTransferCursor(
				rawCursor,
			)

		if err != nil {
			return domain.TransferQuery{}, NewBadInputError("invalid transfer cursor", err)
		}

		query.Cursor =
			&cursor
	}

	return query, nil
}

func (s *HTTPServer) writeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"

	var publicError *PublicError
	switch {
	case errors.As(err, &publicError):
		status = publicError.StatusCode
		message = publicError.Message
		if status == http.StatusBadRequest {
			code = "bad_request"
		}
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "resource not found"
	case errors.Is(err, domain.ErrTemporaryDependency):
		status = http.StatusServiceUnavailable
		code = "service_unavailable"
		message = "service temporarily unavailable"
	}

	if status >= http.StatusInternalServerError {
		s.logger.ErrorContext(r.Context(), "HTTP request failed", "error", err)
	}

	if writeErr := writeAPIError(w, r, status, code, message); writeErr != nil {
		s.logger.ErrorContext(r.Context(), "failed to write HTTP error", "error", writeErr)
	}
}

func (s *HTTPServer) writeMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	if err := writeAPIError(
		w,
		r,
		http.StatusMethodNotAllowed,
		"method_not_allowed",
		"method not allowed",
	); err != nil {
		s.logger.ErrorContext(r.Context(), "failed to write method error", "error", err)
	}
}

func (s *HTTPServer) writeJSONResponse(
	r *http.Request,
	w http.ResponseWriter,
	status int,
	value any,
) {
	if err := writeJSON(w, status, value); err != nil {
		s.logger.ErrorContext(r.Context(), "failed to write JSON response", "error", err)
	}
}

func writeAPIError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	code string,
	message string,
) error {
	return writeJSON(w, status, APIErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			RequestID: requestIDFromContext(r.Context()),
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body = append(body, '\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}
