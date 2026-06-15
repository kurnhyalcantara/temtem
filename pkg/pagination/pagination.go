// Package pagination provides shared limit/offset normalization for list
// endpoints.
package pagination

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Page struct {
	Limit  int
	Offset int
}

// Normalize clamps limit/offset to safe bounds.
func Normalize(limit, offset int) Page {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return Page{Limit: limit, Offset: offset}
}
