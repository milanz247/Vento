package support

import (
	"reflect"
	"testing"
)

func TestNormalizePrefix(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"/":       "",
		"api":     "/api",
		"/api":    "/api",
		"/api/":   "/api",
		"//api//": "/api",
	}
	for in, want := range cases {
		if got := NormalizePrefix(in); got != want {
			t.Errorf("NormalizePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJoinPath(t *testing.T) {
	cases := []struct{ prefix, path, want string }{
		{"/api", "/users", "/api/users"},
		{"/api", "users", "/api/users"},
		{"/api", "", "/api"},
		{"", "/users", "/users"},
		{"", "", "/"},
	}
	for _, tc := range cases {
		if got := JoinPath(tc.prefix, tc.path); got != tc.want {
			t.Errorf("JoinPath(%q,%q) = %q, want %q", tc.prefix, tc.path, got, tc.want)
		}
	}
}

func TestJoinChainsDoesNotAliasInputs(t *testing.T) {
	a := []int{1, 2}
	b := []int{3, 4}

	out1 := JoinChains(a, b)
	out2 := JoinChains(a, []int{9})

	if !reflect.DeepEqual(out1, []int{1, 2, 3, 4}) {
		t.Fatalf("unexpected out1: %v", out1)
	}
	if !reflect.DeepEqual(out2, []int{1, 2, 9}) {
		t.Fatalf("unexpected out2: %v (aliasing bug: reusing a's backing array)", out2)
	}
}
