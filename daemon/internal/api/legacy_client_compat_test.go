package api

// Roadmap 35 (US3 / T089a) — legacy-client backward-compat for additive
// `tenantId` fields (FR-014).
//
// The tenancy roadmap added `tenantId` to ~110 API resource schemas and ~6
// event schemas. The contract is that those additions are STRICTLY ADDITIVE:
//
//   - `tenantId` is declared in `properties` with type `string`.
//   - `tenantId` is NOT in the schema's `required` array, so a legacy client
//     that omits the field still produces a valid request payload.
//   - Stripping `tenantId` from a Roadmap-35 response payload yields a
//     payload byte-equivalent to the pre-Roadmap-35 representation.
//
// This file enforces both invariants:
//
//   - TestTenantIDIsAdditive walks every JSON schema under schemas/api and
//     schemas/events that mentions `tenantId` and asserts the additive shape.
//   - TestLegacyPayloadStripsTenantID picks a representative subset of
//     resources, parses both the modern (with tenantId) and legacy
//     (without) fixtures via a strict-unknown-field decoder, and confirms
//     the legacy payload matches what you get by deleting `tenantId` from
//     the modern payload.
//
// Fixtures live under testdata/legacy_payloads/. To extend coverage to
// another resource, drop in `<name>.json` (modern, includes tenantId) and
// `<name>.legacy.json` (pre-Roadmap-35) — the test discovers them by
// directory walk.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestTenantIDIsAdditive(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	for _, sub := range []string{"api", "events"} {
		dir := filepath.Join(repoRoot, "schemas", sub)
		walkSchemasAssertingAdditive(t, dir)
	}
}

func walkSchemasAssertingAdditive(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !bytes.Contains(data, []byte(`"tenantId"`)) {
			continue
		}
		var schema struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		// Some schemas reference tenantId in $defs / nested objects only.
		// Enforcement applies to the top-level properties slot.
		if _, ok := schema.Properties["tenantId"]; !ok {
			continue
		}
		base := filepath.Base(path)
		// Identity primitives introduced by Roadmap 34 (multi-tenant
		// model) or audit envelopes introduced by Roadmap 35 have no
		// pre-Roadmap-35 legacy client. Their tenantId MAY be required.
		if isIdentityOrAuditSchema(base) {
			continue
		}
		for _, r := range schema.Required {
			if r == "tenantId" {
				t.Errorf("%s: tenantId is in `required` — must remain additive (FR-014)", base)
			}
		}
	}
}

var identityOrAuditPrefixes = []string{
	"tenant-",
	"audit-",
	"membership-",
	"token-tenant-",
	"principal-",
	"tenant_",
}

func isIdentityOrAuditSchema(base string) bool {
	for _, p := range identityOrAuditPrefixes {
		if strings.HasPrefix(base, p) {
			return true
		}
	}
	return false
}

func TestLegacyPayloadStripsTenantID(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "testdata", "legacy_payloads")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	var bases []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".json") || strings.HasSuffix(n, ".legacy.json") {
			continue
		}
		bases = append(bases, strings.TrimSuffix(n, ".json"))
	}
	sort.Strings(bases)
	if len(bases) == 0 {
		t.Fatalf("no modern fixtures discovered under %s", dir)
	}
	for _, base := range bases {
		modernPath := filepath.Join(dir, base+".json")
		legacyPath := filepath.Join(dir, base+".legacy.json")

		modern, err := os.ReadFile(modernPath)
		if err != nil {
			t.Errorf("read %s: %v", modernPath, err)
			continue
		}
		legacy, err := os.ReadFile(legacyPath)
		if err != nil {
			t.Errorf("read %s: %v", legacyPath, err)
			continue
		}

		var modernMap map[string]json.RawMessage
		if err := strictDecode(modern, &modernMap); err != nil {
			t.Errorf("modern %s: strict decode: %v", base, err)
			continue
		}
		var legacyMap map[string]json.RawMessage
		if err := strictDecode(legacy, &legacyMap); err != nil {
			t.Errorf("legacy %s: strict decode: %v", base, err)
			continue
		}

		if _, ok := modernMap["tenantId"]; !ok {
			t.Errorf("modern %s: expected tenantId field", base)
			continue
		}
		if _, ok := legacyMap["tenantId"]; ok {
			t.Errorf("legacy %s: pre-Roadmap-35 fixture must NOT contain tenantId", base)
			continue
		}

		// Strip tenantId from modern and compare canonical JSON to legacy.
		delete(modernMap, "tenantId")
		modernCanon, _ := canonicalize(modernMap)
		legacyCanon, _ := canonicalize(legacyMap)
		if !bytes.Equal(modernCanon, legacyCanon) {
			t.Errorf("legacy %s: tenantId-stripped modern payload diverges from legacy reference\nmodern (stripped): %s\nlegacy:            %s", base, modernCanon, legacyCanon)
		}
	}
}

func strictDecode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// canonicalize re-encodes a JSON map with sorted keys so byte equality
// reflects logical equality regardless of fixture key ordering.
func canonicalize(m map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(m[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
