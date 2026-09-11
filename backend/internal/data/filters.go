package data

import (
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Filters carries the pagination cursor, page size and sort options for a list query.
type Filters struct {
	Cursor    string
	Limit     int
	SortField string
	SortOrder string
}

// Metadata describes a page of paginated results: whether more items exist and the cursor to fetch them.
type Metadata struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	TotalCount int    `json:"total_count,omitempty"`
}

// ValidateFilters checks that f.Limit and f.SortOrder are within their allowed ranges.
func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Limit > 0, "limit", "must be greater than zero")
	v.Check(f.Limit <= 200, "limit", "must be a maximum of 200")
	v.Check(f.SortOrder == "" || f.SortOrder == "ASC" || f.SortOrder == "DESC", "sort_order", "must be ASC or DESC")
}

// ParseCursor decodes f.Cursor into its (time, ID) components, returning
// ErrInvalidCursor if the cursor is malformed.
func (f Filters) ParseCursor() (time.Time, uuid.UUID, error) {
	if f.Cursor == "" {
		return time.Time{}, uuid.Nil, nil
	}

	// UUID is always 36 chars at the end, preceded by ':'
	if len(f.Cursor) < 38 {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}

	uuidStr := f.Cursor[len(f.Cursor)-36:]
	id, err := uuid.Parse(uuidStr)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}

	sep := f.Cursor[len(f.Cursor)-37]
	if sep != ':' {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}

	timeStr := f.Cursor[:len(f.Cursor)-37]

	// Try RFC3339 first (more specific), then date-only.
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		t, err = time.Parse("2006-01-02", timeStr)
		if err != nil {
			return time.Time{}, uuid.Nil, ErrInvalidCursor
		}
	}

	return t, id, nil
}

// CursorKeyer is implemented by paginated result types to provide their cursor key.
type CursorKeyer interface {
	CursorKey() (time.Time, uuid.UUID)
}

// TrimPage applies cursor-based pagination trim to a slice fetched with limit+1.
// It returns the trimmed slice and metadata with the next cursor if more items exist.
func TrimPage[T CursorKeyer](items []T, limit int, buildCursor func(time.Time, uuid.UUID) string) ([]T, Metadata) {
	meta := Metadata{}
	if len(items) > limit {
		meta.HasMore = true
		ts, id := items[limit-1].CursorKey()
		meta.NextCursor = buildCursor(ts, id)
		items = items[:limit]
	}
	return items, meta
}

// BuildNextCursor encodes a date and ID into a date-precision pagination cursor string.
func BuildNextCursor(date time.Time, id uuid.UUID) string {
	return date.Format("2006-01-02") + ":" + id.String()
}

// BuildTimestampCursor encodes a timestamp and ID into an RFC3339-precision pagination cursor string.
func BuildTimestampCursor(ts time.Time, id uuid.UUID) string {
	return ts.UTC().Format(time.RFC3339) + ":" + id.String()
}
