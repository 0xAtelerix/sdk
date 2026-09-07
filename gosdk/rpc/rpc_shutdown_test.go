package rpc

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	return addr
}

func waitForHealth(t *testing.T, addr string) {
	t.Helper()

	client := &http.Client{Timeout: time.Second}

	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://"+addr+"/health",
			nil,
		)
		if err != nil {
			return false
		}

		resp, err := client.Do(req)
		if err != nil {
			return false
		}

		_ = resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
}

func TestStartHTTPServerReturnsOnContextCancel(t *testing.T) {
	t.Parallel()

	server := NewStandardRPCServer(nil)
	addr := freeLoopbackAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)

	go func() { done <- server.StartHTTPServer(ctx, addr) }()

	waitForHealth(t, addr)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("StartHTTPServer did not return after context cancellation")
	}

	dialer := &net.Dialer{Timeout: 200 * time.Millisecond}

	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		_ = conn.Close()
	}

	require.Error(t, err, "listener still accepting after shutdown")
}

func TestStartHTTPServerDrainsInFlightRequestOnCancel(t *testing.T) {
	t.Parallel()

	server := NewStandardRPCServer(nil)
	entered := make(chan struct{})
	release := make(chan struct{})

	server.AddMethod("slow", func(context.Context, []any) (any, error) {
		close(entered)
		<-release

		return "done", nil
	})

	addr := freeLoopbackAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)

	go func() { done <- server.StartHTTPServer(ctx, addr) }()

	waitForHealth(t, addr)

	body, err := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "slow",
		Params:  []any{},
		ID:      1,
	})
	require.NoError(t, err)

	type result struct {
		status int
		body   string
		err    error
	}

	resultCh := make(chan result, 1)

	go func() {
		req, reqErr := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://"+addr+"/rpc",
			bytes.NewReader(body),
		)
		if reqErr != nil {
			resultCh <- result{err: reqErr}

			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, doErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if doErr != nil {
			resultCh <- result{err: doErr}

			return
		}

		defer func() { _ = resp.Body.Close() }()

		raw, _ := io.ReadAll(resp.Body)
		resultCh <- result{status: resp.StatusCode, body: string(raw)}
	}()

	// The handler is running when shutdown begins.
	<-entered
	cancel()

	select {
	case <-done:
		t.Fatal("StartHTTPServer returned while a request was in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	res := <-resultCh
	require.NoError(t, res.err)
	require.Equal(t, http.StatusOK, res.status)
	require.Contains(t, res.body, `"done"`)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("StartHTTPServer did not return after the in-flight request drained")
	}
}
