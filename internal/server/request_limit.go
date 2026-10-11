package server

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

const (
	maxGraphQLRequestBytes   = 8 * 1024 * 1024
	graphQLRequestTooLarge   = "GraphQL request body is too large"
	graphQLRequestUnreadable = "Cannot read GraphQL request"
)

// boundedGraphQLBody rejects oversized POSTs before gqlgen parses their JSON.
func boundedGraphQLBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		if r.ContentLength > maxGraphQLRequestBytes {
			http.Error(w, graphQLRequestTooLarge, http.StatusRequestEntityTooLarge)
			return
		}

		body := http.MaxBytesReader(w, r.Body, maxGraphQLRequestBytes)
		data, err := io.ReadAll(body)
		closeErr := body.Close()

		if err != nil {
			var limitError *http.MaxBytesError
			if errors.As(err, &limitError) {
				http.Error(w, graphQLRequestTooLarge, http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, graphQLRequestUnreadable, http.StatusBadRequest)
			}

			return
		}

		if closeErr != nil {
			http.Error(w, graphQLRequestUnreadable, http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(data))
		next.ServeHTTP(w, r)
	})
}
