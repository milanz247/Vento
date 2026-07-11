package vento

import "testing"

// The clamping/edge-case math itself is tested directly in
// vento/support/pagination_test.go, with no GORM/DB involved. This just
// checks Paginate is wired up and returns a usable GORM scope.
func TestPaginateReturnsScope(t *testing.T) {
	if Paginate(1, 20) == nil {
		t.Fatal("expected Paginate to return a non-nil GORM scope")
	}
}
