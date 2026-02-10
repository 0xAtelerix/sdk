package gosdk

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/goccy/go-json"
)

// CEXPairSubscription represents a single CEX pair that should be polled.
type CEXPairSubscription struct {
	Exchange string `json:"exchange"`
	Symbol   string `json:"symbol"`
}

// CEXPairSubscriptionsPath returns the canonical path for the CEX pair subscriptions file.
func CEXPairSubscriptionsPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), "cex_pair_subscriptions.json")
}

// ReadCEXPairSubscriptions reads the JSON file. Returns nil, nil if the file does not exist.
func ReadCEXPairSubscriptions(filePath string) ([]CEXPairSubscription, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read cex subscriptions: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var subs []CEXPairSubscription
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, fmt.Errorf("unmarshal cex subscriptions: %w", err)
	}

	return subs, nil
}

// AppendCEXPairSubscriptions reads the current file, dedup-merges new pairs,
// sorts deterministically, and writes back via temp+rename (atomic).
func AppendCEXPairSubscriptions(filePath string, newPairs []CEXPairSubscription) error {
	if len(newPairs) == 0 {
		return nil
	}

	existing, err := ReadCEXPairSubscriptions(filePath)
	if err != nil {
		return err
	}

	seen := make(map[CEXPairSubscription]struct{}, len(existing))
	for _, p := range existing {
		seen[p] = struct{}{}
	}

	for _, p := range newPairs {
		if _, ok := seen[p]; !ok {
			existing = append(existing, p)
			seen[p] = struct{}{}
		}
	}

	sort.Slice(existing, func(i, j int) bool {
		if existing[i].Exchange != existing[j].Exchange {
			return existing[i].Exchange < existing[j].Exchange
		}

		return existing[i].Symbol < existing[j].Symbol
	})

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cex subscriptions: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cex subscriptions dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "cex_pair_subscriptions-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)

		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)

		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)

		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
