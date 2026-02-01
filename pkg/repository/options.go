package repository

import "errors"

// ListOptions defines options for listing entities with filtering, sorting, and pagination
type ListOptions struct {
	// Pagination
	Offset int `json:"offset"` // Number of records to skip
	Limit  int `json:"limit"`  // Maximum number of records to return

	// Sorting
	OrderBy   string      `json:"order_by"`   // Field to sort by (e.g., "created_at", "score")
	OrderDesc bool        `json:"order_desc"` // Sort in descending order
	MultiSort []SortField `json:"multi_sort"` // Multiple field sorting

	// Filtering
	Filters map[string]interface{} `json:"filters"` // Generic filters (field -> value)

	// Advanced Filtering
	Where      string        `json:"where"`       // Raw SQL WHERE clause (use with caution)
	WhereArgs  []interface{} `json:"where_args"`  // Arguments for WHERE clause
	OrFilters  []Filter      `json:"or_filters"`  // OR conditions
	AndFilters []Filter      `json:"and_filters"` // AND conditions

	// Query Options
	SelectFields []string `json:"select_fields"` // Specific fields to select (empty = all fields)
	Preload      []string `json:"preload"`       // Relations to preload (for ORMs)
	Joins        []string `json:"joins"`         // Tables to join

	// Performance
	UseCache   bool `json:"use_cache"`   // Use cache if available
	CacheTTL   int  `json:"cache_ttl"`   // Cache TTL in seconds
	NoCount    bool `json:"no_count"`    // Skip total count query (performance optimization)
	ForUpdate  bool `json:"for_update"`  // SELECT ... FOR UPDATE (pessimistic locking)
	SkipLocked bool `json:"skip_locked"` // SKIP LOCKED (skip locked rows)

	// Soft Delete
	IncludeDeleted bool `json:"include_deleted"` // Include soft-deleted records
}

// SortField represents a single field for multi-field sorting
type SortField struct {
	Field string `json:"field"` // Field name
	Desc  bool   `json:"desc"`  // Sort in descending order
}

// Filter represents a single filter condition
type Filter struct {
	Field    string      `json:"field"`    // Field name
	Operator string      `json:"operator"` // Operator: "=", "!=", ">", "<", ">=", "<=", "LIKE", "IN", "NOT IN", "IS NULL", "IS NOT NULL"
	Value    interface{} `json:"value"`    // Filter value
}

// Common filter operators
const (
	OpEqual              = "="
	OpNotEqual           = "!="
	OpGreaterThan        = ">"
	OpLessThan           = "<"
	OpGreaterThanOrEqual = ">="
	OpLessThanOrEqual    = "<="
	OpLike               = "LIKE"
	OpIn                 = "IN"
	OpNotIn              = "NOT IN"
	OpIsNull             = "IS NULL"
	OpIsNotNull          = "IS NOT NULL"
	OpBetween            = "BETWEEN"
)

// Validate validates the ListOptions and sets defaults
func (o *ListOptions) Validate() error {
	// Set default limit
	if o.Limit <= 0 {
		o.Limit = 20
	}

	// Maximum limit to prevent abuse
	if o.Limit > 1000 {
		return errors.New("limit exceeds maximum allowed value of 1000")
	}

	// Offset must be non-negative
	if o.Offset < 0 {
		return errors.New("offset must be non-negative")
	}

	// Validate filter operators
	for _, f := range o.AndFilters {
		if err := f.Validate(); err != nil {
			return err
		}
	}

	for _, f := range o.OrFilters {
		if err := f.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates a filter condition
func (f *Filter) Validate() error {
	if f.Field == "" {
		return errors.New("filter field cannot be empty")
	}

	validOperators := map[string]bool{
		OpEqual:              true,
		OpNotEqual:           true,
		OpGreaterThan:        true,
		OpLessThan:           true,
		OpGreaterThanOrEqual: true,
		OpLessThanOrEqual:    true,
		OpLike:               true,
		OpIn:                 true,
		OpNotIn:              true,
		OpIsNull:             true,
		OpIsNotNull:          true,
		OpBetween:            true,
	}

	if !validOperators[f.Operator] {
		return errors.New("invalid filter operator: " + f.Operator)
	}

	// IS NULL and IS NOT NULL don't need a value
	if f.Operator != OpIsNull && f.Operator != OpIsNotNull && f.Value == nil {
		return errors.New("filter value cannot be nil for operator: " + f.Operator)
	}

	return nil
}

// AddFilter adds a filter condition to the options
func (o *ListOptions) AddFilter(field, operator string, value interface{}) {
	if o.AndFilters == nil {
		o.AndFilters = make([]Filter, 0)
	}
	o.AndFilters = append(o.AndFilters, Filter{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
}

// AddOrFilter adds an OR filter condition to the options
func (o *ListOptions) AddOrFilter(field, operator string, value interface{}) {
	if o.OrFilters == nil {
		o.OrFilters = make([]Filter, 0)
	}
	o.OrFilters = append(o.OrFilters, Filter{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
}

// SetSort sets a single sort field
func (o *ListOptions) SetSort(field string, desc bool) {
	o.OrderBy = field
	o.OrderDesc = desc
}

// AddSort adds a field to multi-field sorting
func (o *ListOptions) AddSort(field string, desc bool) {
	if o.MultiSort == nil {
		o.MultiSort = make([]SortField, 0)
	}
	o.MultiSort = append(o.MultiSort, SortField{
		Field: field,
		Desc:  desc,
	})
}

// SetPagination sets pagination parameters
func (o *ListOptions) SetPagination(page, pageSize int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	o.Offset = (page - 1) * pageSize
	o.Limit = pageSize
}

// PaginationResult represents the result of a paginated query
type PaginationResult[T any] struct {
	Items      []*T  `json:"items"`       // The actual data
	Total      int64 `json:"total"`       // Total number of records
	Page       int   `json:"page"`        // Current page number
	PageSize   int   `json:"page_size"`   // Page size
	TotalPages int   `json:"total_pages"` // Total number of pages
	HasMore    bool  `json:"has_more"`    // Whether there are more pages
}

// NewPaginationResult creates a new pagination result
func NewPaginationResult[T any](items []*T, total int64, opts ListOptions) *PaginationResult[T] {
	page := (opts.Offset / opts.Limit) + 1
	totalPages := int((total + int64(opts.Limit) - 1) / int64(opts.Limit))

	return &PaginationResult[T]{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   opts.Limit,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}
}
