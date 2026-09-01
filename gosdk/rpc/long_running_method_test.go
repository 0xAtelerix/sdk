package rpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

const testRPCOK = "ok"

var (
	errDeadlineUnsupported  = errors.New("deadline unsupported")
	errUnexpectedHTTPStatus = errors.New("unexpected HTTP status")
)

type deadlineRecorder struct {
	header    http.Header
	deadlines []time.Time
	err       error
}

func (w *deadlineRecorder) Header() http.Header     { return w.header }
func (*deadlineRecorder) Write([]byte) (int, error) { return 0, nil }
func (*deadlineRecorder) WriteHeader(int)           {}
func (w *deadlineRecorder) SetWriteDeadline(v time.Time) error {
	w.deadlines = append(w.deadlines, v)

	return w.err
}

func TestLongRunningMethodDeadlineSelection(t *testing.T) {
	t.Parallel()

	server := NewStandardRPCServer(nil)
	logger := zerolog.Nop()
	server.logger = &logger
	handler := func(context.Context, []any) (any, error) { return testRPCOK, nil }
	server.AddLongRunningMethod("deployStrategy", handler)
	server.AddMethod("getStatus", handler)

	long := &deadlineRecorder{header: make(http.Header)}
	require.NoError(t, server.allowLongRunningResponse(long, "deployStrategy"))
	require.Equal(t, []time.Time{{}}, long.deadlines)

	regular := &deadlineRecorder{header: make(http.Header)}
	require.NoError(t, server.allowLongRunningResponse(regular, "getStatus"))
	require.Empty(t, regular.deadlines)

	unsupported := &deadlineRecorder{header: make(http.Header), err: errDeadlineUnsupported}
	require.ErrorIs(t,
		server.allowLongRunningResponse(unsupported, "deployStrategy"),
		errDeadlineUnsupported,
	)
}

func TestLongRunningMethodsCrossThirtySecondHTTPBoundary(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("real >30-second HTTP boundary proof")
	}

	server := NewStandardRPCServer(nil)
	logger := zerolog.Nop()
	server.logger = &logger

	const wait = 30*time.Second + 100*time.Millisecond

	slow := func(context.Context, []any) (any, error) {
		time.Sleep(wait)

		return testRPCOK, nil
	}
	server.AddLongRunningMethod("deployStrategy", slow)
	server.AddLongRunningMethod("strategy_update", slow)

	httpServer := httptest.NewUnstartedServer(server.rpcHandler())
	httpServer.Config.WriteTimeout = 50 * time.Millisecond
	httpServer.Start()
	t.Cleanup(httpServer.Close)

	methods := []string{"deployStrategy", "strategy_update"}

	errs := make(chan error, len(methods))
	for index, method := range methods {
		go func() {
			response, err := callTestRPC(httpServer.URL+"/rpc", method, index+1)
			if err == nil {
				closeErr := response.Body.Close()

				if response.StatusCode != http.StatusOK {
					err = errUnexpectedHTTPStatus
				} else {
					err = closeErr
				}
			}

			errs <- err
		}()
	}

	for range methods {
		require.NoError(t, <-errs)
	}
}

func TestRegularMethodRetainsHTTPWriteTimeout(t *testing.T) {
	t.Parallel()

	server := NewStandardRPCServer(nil)
	logger := zerolog.Nop()
	server.logger = &logger
	server.AddMethod("blocking", func(context.Context, []any) (any, error) {
		time.Sleep(150 * time.Millisecond)

		return "late", nil
	})

	httpServer := httptest.NewUnstartedServer(server.rpcHandler())
	httpServer.Config.WriteTimeout = 25 * time.Millisecond
	httpServer.Start()
	t.Cleanup(httpServer.Close)

	response, err := callTestRPC(httpServer.URL+"/rpc", "blocking", 1)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}

	require.Error(t, err)
}

func TestBatchContainingLongRunningMethodIsRejected(t *testing.T) {
	t.Parallel()

	server := NewStandardRPCServer(nil)
	logger := zerolog.Nop()
	server.logger = &logger

	var calls atomic.Int64

	handler := func(context.Context, []any) (any, error) {
		calls.Add(1)

		return testRPCOK, nil
	}
	server.AddLongRunningMethod("deployStrategy", handler)
	server.AddMethod("getStatus", handler)

	httpServer := httptest.NewServer(server.rpcHandler())
	t.Cleanup(httpServer.Close)

	body, err := json.Marshal([]JSONRPCRequest{
		{JSONRPC: jsonRPCVersion, Method: "deployStrategy", ID: 1},
		{JSONRPC: jsonRPCVersion, Method: "getStatus", ID: 2},
	})
	require.NoError(t, err)

	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		httpServer.URL+"/rpc",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)

	defer func() { require.NoError(t, response.Body.Close()) }()

	var rpcResponse JSONRPCResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&rpcResponse))
	require.Equal(t, -32600, rpcResponse.Error.Code)
	require.Contains(t, rpcResponse.Error.Message, "not allowed in a batch")
	require.Zero(t, calls.Load())
}

func callTestRPC(url, method string, id int) (*http.Response, error) {
	body, err := json.Marshal(JSONRPCRequest{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		ID:      id,
	})
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		closeErr := response.Body.Close()

		return nil, errors.Join(err, closeErr)
	}

	return response, nil
}
