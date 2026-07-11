package vento

import (
	"vento-app/vento/support"

	"gorm.io/gorm"
)

// Pagination page-size defaults. MaxPerPage is a safety ceiling: it stops a
// client from requesting an unbounded result set (e.g. ?per_page=1000000)
// and turning a list endpoint into a memory-exhaustion vector. Override
// them at startup if an app's needs differ.
var (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// Paginate is a GORM scope that applies LIMIT/OFFSET for a 1-based page
// number, pairing directly with c.QueryInt to turn ?page=/&per_page= into a
// paged query with no arithmetic in the handler:
//
//	var users []models.User
//	page := c.QueryInt("page", 1)
//	perPage := c.QueryInt("per_page", vento.DefaultPerPage)
//	c.DB().Scopes(vento.Paginate(page, perPage)).Find(&users)
//
// page < 1 is treated as page 1; perPage < 1 falls back to DefaultPerPage
// and is capped at MaxPerPage, so out-of-range client input can never
// produce a negative offset or an unbounded fetch. The clamping itself
// (support.PaginationBounds) is pure and lives in vento/support so it's
// testable with no GORM/DB involved.
func Paginate(page, perPage int) func(*gorm.DB) *gorm.DB {
	limit, offset := support.PaginationBounds(page, perPage, DefaultPerPage, MaxPerPage)
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(limit)
	}
}
