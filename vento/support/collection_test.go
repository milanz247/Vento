package support

import (
	"reflect"
	"testing"
)

func TestMap(t *testing.T) {
	in := []int{1, 2, 3}
	got := Map(in, func(n int) string {
		if n == 1 {
			return "one"
		}
		return "many"
	})
	want := []string{"one", "many", "many"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Map() = %v, want %v", got, want)
	}
}

func TestMapEmptyInput(t *testing.T) {
	got := Map([]int{}, func(n int) int { return n * 2 })
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestFilter(t *testing.T) {
	in := []int{1, 2, 3, 4, 5, 6}
	got := Filter(in, func(n int) bool { return n%2 == 0 })
	want := []int{2, 4, 6}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter() = %v, want %v", got, want)
	}
}

func TestFilterNoMatchesReturnsNonNilEmptySlice(t *testing.T) {
	got := Filter([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
	if got == nil {
		t.Fatal("expected a non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}
