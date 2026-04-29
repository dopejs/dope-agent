package opsreadiness

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadJSONFixture[T any](path string) (T, error) {
	var out T
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Errorf("read fixture %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode fixture %s: %w", path, err)
	}
	return out, nil
}
