package product

import "errors"

var (
	ErrNotFound        = errors.New("document not found")
	ErrVersionConflict = errors.New("document version conflict")
)

type ProductSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Product struct {
	ID          string        `json:"id" binding:"required"`
	Name        string        `json:"name" binding:"required"`
	Description string        `json:"description"`
	Brand       string        `json:"brand" binding:"required"`
	Category    string        `json:"category" binding:"required"`
	Price       float64       `json:"price" binding:"required"`
	Stock       int           `json:"stock"`
	Rating      float64       `json:"rating"`
	ViewCount   int           `json:"view_count,omitempty"`
	Tags        []string      `json:"tags"`
	Specs       []ProductSpec `json:"specs"`
	CreatedAt   string        `json:"created_at" binding:"required"`
}

type ProductDocument struct {
	ElasticID   string  `json:"elastic_id"`
	SeqNo       int64   `json:"seq_no"`
	PrimaryTerm int64   `json:"primary_term"`
	Product     Product `json:"product"`
}

type ProductUpdateResult struct {
	ID          string `json:"id"`
	Result      string `json:"result"`
	SeqNo       int64  `json:"seq_no"`
	PrimaryTerm int64  `json:"primary_term"`
}

type DecreaseStockRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

type BulkSeedResult struct {
	Index     string          `json:"index"`
	Total    int             `json:"total"`
	HasError bool            `json:"has_error"`
	Errors   []BulkItemError `json:"errors,omitempty"`
}

type BulkItemError struct {
	Operation string         `json:"operation"`
	ID        string         `json:"id"`
	Status    int            `json:"status"`
	Error     map[string]any `json:"error"`
}

type bulkItemResult struct {
	ID     string         `json:"_id"`
	Status int            `json:"status"`
	Error  map[string]any `json:"error,omitempty"`
}

// Product              → document trong Elasticsearch
// ProductSpec          → object con trong specs
// ProductDocument      → response khi GET, có thêm _seq_no và _primary_term
// ProductUpdateResult  → response từ Update API
// BulkSeedResult       → response khi bulk seed