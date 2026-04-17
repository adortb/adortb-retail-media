package catalog

import (
	"testing"
)

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name    string
		product *Product
		wantErr bool
	}{
		{"valid", &Product{SKU: "SKU001", Title: "Test", AdvertiserID: 1}, false},
		{"nil", nil, true},
		{"no sku", &Product{Title: "Test", AdvertiserID: 1}, true},
		{"no title", &Product{SKU: "SKU001", AdvertiserID: 1}, true},
		{"no advertiser", &Product{SKU: "SKU001", Title: "Test"}, true},
	}
	for _, tc := range tests {
		err := validateProduct(tc.product)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: validateProduct error = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestStatusOrDefault(t *testing.T) {
	if got := statusOrDefault(""); got != "active" {
		t.Errorf("statusOrDefault empty = %q, want active", got)
	}
	if got := statusOrDefault("paused"); got != "paused" {
		t.Errorf("statusOrDefault paused = %q, want paused", got)
	}
}

func TestJoinComma(t *testing.T) {
	got := joinComma([]string{"$1", "$2", "$3"})
	if got != "$1,$2,$3" {
		t.Errorf("joinComma = %q, want $1,$2,$3", got)
	}
	if joinComma(nil) != "" {
		t.Error("joinComma nil should return empty")
	}
}
