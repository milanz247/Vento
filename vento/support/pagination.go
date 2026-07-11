// Package support holds pure, dependency-free logic used by the vento
// package - path/prefix math, pagination arithmetic, and similar. It
// exists as its own package (rather than living directly in vento) so this
// logic is physically separated from the framework's HTTP/Context plumbing
// and independently testable with no *vento.Context, *http.Request, or
// database in sight.
//
// It cannot hold vento.Context methods (like c.OK or c.ParamInt) itself -
// Go requires a method to be declared in the same package as its receiver
// type, so anything shaped like "c.Something()" must stay in package
// vento. What moves here is the underlying computation those methods (and
// the framework's routing) delegate to.
package support

// PaginationBounds converts a 1-based page number and a requested page
// size into the sanitized (limit, offset) pair to apply to a query.
//
//   - page < 1 clamps to page 1 (offset 0), so an out-of-range or garbage
//     page number never produces a negative offset.
//   - perPage < 1 falls back to defaultPerPage.
//   - perPage > maxPerPage is capped at maxPerPage, so a client can never
//     force an unbounded fetch (e.g. ?per_page=1000000).
func PaginationBounds(page, perPage, defaultPerPage, maxPerPage int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return perPage, (page - 1) * perPage
}
