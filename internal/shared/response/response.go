package response

import "github.com/gin-gonic/gin"

// Pagination represents pagination metadata
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Meta represents response metadata
type Meta struct {
	Pagination *Pagination `json:"pagination,omitempty"`

	// Counts carries per-bucket totals alongside a page of results — order
	// counts per status, for the FE's tab badges. A map rather than a typed
	// struct because the buckets belong to the endpoint (order statuses
	// here, something else later), not to the envelope.
	//
	// omitempty keeps it absent from every response that does not set it,
	// so adding this field changed no existing response body.
	Counts map[string]int64 `json:"counts,omitempty"`
}

// Response represents standard API response structure
type Response struct {
	Data    interface{}            `json:"data"`
	Message string                 `json:"message"`
	Meta    *Meta                  `json:"meta,omitempty"`
	Errors  map[string][]string    `json:"errors,omitempty"`
}

// Success sends a success response without pagination
func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Data:    data,
		Message: message,
	})
}

// SuccessWithPagination sends a success response with pagination metadata
func SuccessWithPagination(c *gin.Context, statusCode int, message string, data interface{}, page, limit int, total int64) {
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(statusCode, Response{
		Data:    data,
		Message: message,
		Meta: &Meta{
			Pagination: &Pagination{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			},
		},
	})
}

// SuccessWithPaginationAndCounts is SuccessWithPagination plus per-bucket
// totals in meta.counts. Split into its own function rather than added as a
// parameter so that none of the existing call sites change.
//
// counts is expected to cover the whole filtered set and to ignore the
// endpoint's own bucket filter — for orders that means the status counts are
// the same whether or not ?status= was supplied, which is what lets one
// request drive every tab badge. Passing nil is the same as calling
// SuccessWithPagination.
func SuccessWithPaginationAndCounts(c *gin.Context, statusCode int, message string, data interface{}, page, limit int, total int64, counts map[string]int64) {
	totalPages := 0
	if limit > 0 {
		totalPages = int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
	}

	c.JSON(statusCode, Response{
		Data:    data,
		Message: message,
		Meta: &Meta{
			Pagination: &Pagination{
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			},
			Counts: counts,
		},
	})
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, message string, errorDetail string) {
	resp := Response{
		Data:    nil,
		Message: message,
	}

	if errorDetail != "" {
		resp.Errors = map[string][]string{
			"detail": {errorDetail},
		}
	}

	c.JSON(statusCode, resp)
}

// ValidationError sends a validation error response
func ValidationError(c *gin.Context, statusCode int, message string, errors map[string][]string) {
	c.JSON(statusCode, Response{
		Data:    nil,
		Message: message,
		Errors:  errors,
	})
}