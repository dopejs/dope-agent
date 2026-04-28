package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ValueBackend interface {
	Put(ctx context.Context, tenantID, secretID, secretVersionID, value string) (string, error)
	Get(ctx context.Context, backendRef string) (string, error)
	Delete(ctx context.Context, backendRef string) error
}

type LocalBackend struct {
	root string
}

func NewLocalBackend(root string) (*LocalBackend, error) {
	if root == "" {
		return nil, errors.New("secret backend root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create secret backend root: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("chmod secret backend root: %w", err)
	}
	return &LocalBackend{root: root}, nil
}

func (b *LocalBackend) Put(_ context.Context, tenantID, secretID, secretVersionID, value string) (string, error) {
	if b == nil || b.root == "" {
		return "", errors.New("secret backend is not configured")
	}
	if tenantID == "" {
		return "", ErrTenantRequired
	}
	if secretID == "" || secretVersionID == "" {
		return "", errors.New("secret id and version id are required")
	}
	if value == "" {
		return "", ErrSecretValueRequired
	}

	tenantDir := filepath.Join(b.root, safePathSegment(tenantID))
	if err := os.MkdirAll(tenantDir, 0o700); err != nil {
		return "", fmt.Errorf("create tenant secret dir: %w", err)
	}
	name := safePathSegment(secretID) + "_" + safePathSegment(secretVersionID) + "_" + randomHex(8)
	path := filepath.Join(tenantDir, name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return "", fmt.Errorf("write secret value: %w", err)
	}
	return filepath.ToSlash(filepath.Join(safePathSegment(tenantID), name)), nil
}

func (b *LocalBackend) Get(_ context.Context, backendRef string) (string, error) {
	if b == nil || b.root == "" {
		return "", errors.New("secret backend is not configured")
	}
	if backendRef == "" {
		return "", ErrSecretVersionNotFound
	}
	path := filepath.Clean(filepath.Join(b.root, filepath.FromSlash(backendRef)))
	root := filepath.Clean(b.root)
	if path != root && !hasPathPrefix(path, root) {
		return "", errors.New("secret backend ref escapes root")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret value: %w", err)
	}
	return string(data), nil
}

func (b *LocalBackend) Delete(_ context.Context, backendRef string) error {
	if b == nil || b.root == "" || backendRef == "" {
		return nil
	}
	path := filepath.Clean(filepath.Join(b.root, filepath.FromSlash(backendRef)))
	root := filepath.Clean(b.root)
	if path != root && !hasPathPrefix(path, root) {
		return errors.New("secret backend ref escapes root")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete secret value: %w", err)
	}
	return nil
}

func safePathSegment(value string) string {
	out := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "_"
	}
	return string(out)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

func hasPathPrefix(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && len(rel) > 0 && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
