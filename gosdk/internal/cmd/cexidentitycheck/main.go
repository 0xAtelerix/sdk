// Command cexidentitycheck validates source-controlled CEX identity registry evolution.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/internal/cexidentity"
)

var (
	errRequiredArguments = errors.New("base-ref and current are required")
	errOutsideRepository = errors.New("current registry is outside repository")
)

func main() {
	baseRef := flag.String("base-ref", "origin/main", "git ref containing the base registry")
	currentPath := flag.String("current", "", "path to the current registry JSON")

	flag.Parse()

	if err := run(context.Background(), *baseRef, *currentPath); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}

		os.Exit(1)
	}

	if _, err := fmt.Println("cex identity registry evolution: PASS"); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, baseRef, currentPath string) error {
	if strings.TrimSpace(baseRef) == "" || strings.TrimSpace(currentPath) == "" {
		return errRequiredArguments
	}

	root, err := gitOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("find git root: %w", err)
	}

	gitPath, err := repositoryRelativePath(root, currentPath)
	if err != nil {
		return err
	}

	current, err := readDocument(currentPath)
	if err != nil {
		return fmt.Errorf("read current registry: %w", err)
	}

	if _, err = gitOutput(ctx, "rev-parse", "--verify", baseRef+"^{commit}"); err != nil {
		return fmt.Errorf("verify base ref %q: %w", baseRef, err)
	}

	baseBytes, exists, err := readBaseDocument(ctx, baseRef, gitPath)
	if err != nil {
		return err
	}

	if !exists {
		return cexidentity.ValidateInitial(current)
	}

	var previous apptypes.OrderBookIdentityJSON
	if err := json.Unmarshal(baseBytes, &previous); err != nil {
		return fmt.Errorf("decode base registry %s:%s: %w", baseRef, gitPath, err)
	}

	return cexidentity.ValidateEvolution(previous, current)
}

func repositoryRelativePath(root, currentPath string) (string, error) {
	absolutePath, err := filepath.Abs(currentPath)
	if err != nil {
		return "", fmt.Errorf("resolve current registry path: %w", err)
	}

	relativePath, err := filepath.Rel(strings.TrimSpace(root), absolutePath)
	if err != nil {
		return "", fmt.Errorf("make current registry path relative: %w", err)
	}

	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", errOutsideRepository, currentPath)
	}

	return filepath.ToSlash(relativePath), nil
}

func readDocument(path string) (apptypes.OrderBookIdentityJSON, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return apptypes.OrderBookIdentityJSON{}, err
	}

	var document apptypes.OrderBookIdentityJSON
	if err := json.Unmarshal(contents, &document); err != nil {
		return apptypes.OrderBookIdentityJSON{}, err
	}

	return document, nil
}

func readBaseDocument(ctx context.Context, baseRef, gitPath string) ([]byte, bool, error) {
	object := baseRef + ":" + gitPath

	contents, err := gitOutput(ctx, "show", object)
	if err == nil {
		return []byte(contents), true, nil
	}

	listing, listErr := gitOutput(ctx, "ls-tree", "-r", "--name-only", baseRef, "--", gitPath)
	if listErr != nil {
		return nil, false, fmt.Errorf("inspect base registry %s: %w", object, listErr)
	}

	if strings.TrimSpace(listing) == "" {
		return nil, false, nil
	}

	return nil, false, fmt.Errorf("read base registry %s: %w", object, err)
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	// Output() returns stdout only, so a stderr warning on an otherwise-successful
	// `git show` cannot corrupt the JSON bytes fed to the decoder. Stderr is still
	// surfaced from the exit error when the command fails.
	output, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		stderr := ""

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}

		return "", fmt.Errorf(
			"git %s: %w: %s",
			strings.Join(args, " "),
			err,
			stderr,
		)
	}

	return strings.TrimSpace(string(output)), nil
}
