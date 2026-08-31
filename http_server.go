package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

type HTTPServer struct {
	transferReader        TransferReader
	tokenMetadataProvider TokenMetadataProvider

	logger  *slog.Logger
	metrics *Metrics
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
		NewMetrics(),
	)
}

func NewHTTPServerWithObservability(
	transferReader TransferReader,
	tokenMetadataProvider TokenMetadataProvider,
	logger *slog.Logger,
	metrics *Metrics,
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
			NewMetrics()
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

	return observabilityMiddleware(
		s.logger,
		s.metrics,
		mux,
	)
}

func (s *HTTPServer) handleHealth(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	writeJSON(
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
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

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
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

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

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

func parseTransferQuery(
	r *http.Request,
) (TransferQuery, error) {
	values :=
		r.URL.Query()

	query :=
		TransferQuery{
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
			return TransferQuery{}, NewBadInputError("invalid block", err)
		}

		query.BlockNumber =
			&blockNumber
	}

	if rawToken :=
		values.Get("token"); rawToken != "" {

		token :=
			Address(rawToken)

		query.Token =
			&token
	}

	if rawAddress :=
		values.Get("address"); rawAddress != "" {

		address :=
			Address(rawAddress)

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

			return TransferQuery{}, NewBadInputError("invalid limit", errors.New("limit must be a positive integer"))
		}

		if limit > 1000 {
			return TransferQuery{}, NewBadInputError("limit cannot exceed 1000", errors.New("limit exceeds maximum"))
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
			return TransferQuery{}, NewBadInputError("invalid transfer cursor", err)
		}

		query.Cursor =
			&cursor
	}

	return query, nil
}

func (s *HTTPServer) writeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	var publicError *PublicError
	switch {
	case errors.As(err, &publicError):
		status = publicError.StatusCode
		message = publicError.Message
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
		message = "resource not found"
	case errors.Is(err, ErrTemporaryDependency):
		status = http.StatusServiceUnavailable
		message = "service temporarily unavailable"
	}

	if status >= http.StatusInternalServerError {
		s.logger.ErrorContext(r.Context(), "HTTP request failed", "error", err)
	}

	http.Error(w, message, status)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(
		status,
	)

	if err :=
		json.NewEncoder(w).
			Encode(value); err != nil {

		fmt.Println(
			"failed to write JSON response:",
			err,
		)
	}
}
