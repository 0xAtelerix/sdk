package rpc

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type deadlineRecorder struct {
	header    http.Header
	deadlines []time.Time
}

func (w *deadlineRecorder) Header() http.Header     { return w.header }
func (*deadlineRecorder) Write([]byte) (int, error) { return 0, nil }
func (*deadlineRecorder) WriteHeader(int)           {}
func (w *deadlineRecorder) SetWriteDeadline(v time.Time) error {
	w.deadlines = append(w.deadlines, v)
	return nil
}

func TestLongRunningMethodDisablesOnlyItsWriteDeadline(t *testing.T) {
	server := NewStandardRPCServer(nil)
	handler := func(context.Context, []any) (any, error) { return "ok", nil }
	server.AddLongRunningMethod("deployStrategy", handler)
	server.AddMethod("getStatus", handler)

	long := &deadlineRecorder{header: make(http.Header)}
	server.allowLongRunningResponse(long, "deployStrategy")
	require.Equal(t, []time.Time{{}}, long.deadlines)

	regular := &deadlineRecorder{header: make(http.Header)}
	server.allowLongRunningResponse(regular, "getStatus")
	require.Empty(t, regular.deadlines)
}
