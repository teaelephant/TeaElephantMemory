package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/require"
)

func TestGraphQLWebsocketProtocolsAcceptCrossOriginClients(t *testing.T) {
	t.Parallel()

	const legacyProtocol = "graphql-ws"
	for _, protocol := range []string{legacyProtocol, "graphql-transport-ws"} {
		t.Run(protocol, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			s := NewServer(nil, nil, nil)
			s.InitV2Api()
			httpServer := httptest.NewServer(s.router)
			t.Cleanup(httpServer.Close)

			url := strings.Replace(httpServer.URL, "http://", "ws://", 1) + v2QueryPath
			conn, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{
				Subprotocols: []string{protocol},
				HTTPHeader:   http.Header{"Origin": {"https://client.example"}},
			})

			if response != nil && response.Body != nil {
				require.NoError(t, response.Body.Close())
			}

			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, conn.CloseNow()) })

			type request struct {
				Type    string            `json:"type"`
				ID      string            `json:"id,omitempty"`
				Payload map[string]string `json:"payload,omitempty"`
			}
			require.NoError(t, wsjson.Write(ctx, conn, request{Type: "connection_init"}))

			var message struct {
				Type    string `json:"type"`
				Payload struct {
					Data map[string]string `json:"data"`
				} `json:"payload"`
			}
			require.NoError(t, wsjson.Read(ctx, conn, &message))
			require.Equal(t, "connection_ack", message.Type)

			messageType, responseType := "subscribe", "next"
			if protocol == legacyProtocol {
				messageType, responseType = "start", "data"
			}

			require.NoError(t, wsjson.Write(ctx, conn, request{
				ID:      "smoke",
				Type:    messageType,
				Payload: map[string]string{"query": "query { __typename }"},
			}))

			for {
				require.NoError(t, wsjson.Read(ctx, conn, &message))

				if message.Type != "ka" {
					break
				}
			}

			require.Equal(t, responseType, message.Type)
			require.Equal(t, "Query", message.Payload.Data["__typename"])
		})
	}
}
