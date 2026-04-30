package evaluation

import "testing"

func TestValidateTenantScopedProductRequestRequiresTenant(t *testing.T) {
	if err := ValidateTenantScopedProductRequest(""); err == nil {
		t.Fatal("expected missing tenant to fail")
	}
	if err := ValidateTenantScopedProductRequest("ten_eval"); err != nil {
		t.Fatalf("expected tenant to pass: %v", err)
	}
}

func TestNormalizeProductLimitBoundsLists(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default", in: 0, want: DefaultProductPageLimit},
		{name: "negative", in: -1, want: DefaultProductPageLimit},
		{name: "keeps explicit", in: 25, want: 25},
		{name: "caps max", in: MaxProductPageLimit + 1, want: MaxProductPageLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeProductLimit(tt.in); got != tt.want {
				t.Fatalf("NormalizeProductLimit(%d)=%d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
