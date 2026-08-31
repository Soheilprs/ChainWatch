package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type HTTPServer struct {
	transferReader TransferReader
}

func NewHTTPServer(
	transferReader TransferReader,
) *HTTPServer {
	return &HTTPServer{
		transferReader: transferReader,
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

	return mux
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
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)

		return
	}

	page, err :=
		s.transferReader.ListTransfers(
			r.Context(),
			query,
		)

	if err != nil {
		http.Error(
			w,
			"failed to load transfers",
			http.StatusInternalServerError,
		)

		return
	}

	data := make(
		[]APITransfer,
		0,
		len(page.Transfers),
	)

	for _, transfer := range page.Transfers {

		data = append(
			data,
			apiTransferFromStored(
				transfer,
			),
		)
	}

	var nextCursor *string

	if page.NextCursor != nil {
		encodedCursor, err :=
			EncodeTransferCursor(
				*page.NextCursor,
			)

		if err != nil {
			http.Error(
				w,
				"failed to encode pagination cursor",
				http.StatusInternalServerError,
			)

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

	query := TransferQuery{
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
			return TransferQuery{},
				errors.New(
					"invalid block",
				)
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

			return TransferQuery{},
				errors.New(
					"invalid limit",
				)
		}

		if limit > 1000 {
			return TransferQuery{},
				errors.New(
					"limit cannot exceed 1000",
				)
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
			return TransferQuery{},
				ErrInvalidTransferCursor
		}

		query.Cursor =
			&cursor
	}

	return query, nil
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
