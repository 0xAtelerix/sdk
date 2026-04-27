package gosdk

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventReaderBlockingUsesConfiguredPollIntervalWhenWatcherIsQuiet(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "epoch_1.data")
	require.NoError(t, os.WriteFile(path, eventReaderHeader(), 0o644))

	f, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	require.NoError(t, err)

	reader := &EventReader{
		dataFile:     f,
		pollInterval: 5 * time.Millisecond,
		position:     8,
	}

	t.Cleanup(func() {
		require.NoError(t, reader.Close())
	})

	var atropos [32]byte

	atropos[31] = 1

	go func() {
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, appendEventReaderBatch(path, atropos, [][]byte{[]byte("event")}))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	batches, err := reader.GetNewBatchesBlocking(ctx, 10)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, batches, 1)
	require.Equal(t, atropos, batches[0].Atropos)
	require.Equal(t, [][]byte{[]byte("event")}, batches[0].Events)
	require.Less(t, elapsed, 90*time.Millisecond)
}

func eventReaderHeader() []byte {
	header := make([]byte, 8)
	binary.BigEndian.PutUint64(header, math.MaxUint64)

	return header
}

func appendEventReaderBatch(path string, atropos [32]byte, events [][]byte) error {
	var size uint32 = 32
	for _, event := range events {
		size += 4 + uint32(len(event))
	}

	buf := make([]byte, 4+size)
	binary.BigEndian.PutUint32(buf[:4], size)
	copy(buf[4:36], atropos[:])

	offset := 36
	for _, event := range events {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(len(event)))
		offset += 4
		copy(buf[offset:offset+len(event)], event)
		offset += len(event)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(buf); err != nil {
		return err
	}

	return f.Sync()
}
