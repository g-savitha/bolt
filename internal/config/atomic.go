package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const privateFileMode = 0o600

// writeAtomic writes path by encoding to a temp file in the same directory,
// syncing to disk, then renaming into place. A crash leaves path unchanged
// or fully replaced — never empty or half-written.
func writeAtomic(path string, encode func(w io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, privateFileMode) //nolint:gosec // G304: path derived from config dir
	if err != nil {
		return fmt.Errorf("open temp file %s: %w", tmp, err)
	}

	writeErr := func() error {
		if err := encode(f); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("sync temp file: %w", err)
		}
		return nil
	}()

	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp file: %w", closeErr)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s to %s: %w", tmp, path, err)
	}
	if err := os.Chmod(path, privateFileMode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}
