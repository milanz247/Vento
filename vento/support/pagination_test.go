package support

import "testing"

func TestPaginationBounds(t *testing.T) {
	const defaultPerPage, maxPerPage = 20, 100

	cases := []struct {
		page, perPage      int
		wantLimit, wantOff int
	}{
		{1, 20, 20, 0},      // first page
		{2, 20, 20, 20},     // second page offsets by one page
		{3, 15, 15, 30},     // custom page size
		{0, 20, 20, 0},      // page < 1 clamps to page 1 (offset 0)
		{-5, 20, 20, 0},     // negative page clamps to page 1
		{2, 0, 20, 20},      // perPage < 1 falls back to defaultPerPage
		{2, -3, 20, 20},     // negative perPage falls back to defaultPerPage
		{2, 1000, 100, 100}, // perPage over maxPerPage is capped
	}
	for _, tc := range cases {
		limit, offset := PaginationBounds(tc.page, tc.perPage, defaultPerPage, maxPerPage)
		if limit != tc.wantLimit || offset != tc.wantOff {
			t.Errorf("PaginationBounds(%d,%d,%d,%d) = (limit %d, offset %d); want (%d, %d)",
				tc.page, tc.perPage, defaultPerPage, maxPerPage, limit, offset, tc.wantLimit, tc.wantOff)
		}
	}
}
