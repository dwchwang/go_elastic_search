package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func (m *Module) SeedProducts(c *gin.Context) {
	result, err := m.seedProducts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	status := http.StatusOK
	if result.HasError {
		status = http.StatusMultiStatus
	}

	c.JSON(status, gin.H{
		"message": "products seed completed",
		"result":  result,
	})
}

func (m *Module) seedProducts() (*BulkSeedResult, error) {
	data, err := os.ReadFile("data/products.ndjson")
	if err != nil {
		return nil, fmt.Errorf("read seed file failed: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("seed file is empty")
	}

	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	res, err := m.es.Bulk(
		bytes.NewReader(data),
		m.es.Bulk.WithIndex(IndexName),
		m.es.Bulk.WithRefresh("true"),
	)
	if err != nil {
		return nil, fmt.Errorf("bulk seed products failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, readESError("bulk seed products", res.Body)
	}

	var bulkResp struct {
		Errors bool                        `json:"errors"`
		Items  []map[string]bulkItemResult `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		return nil, fmt.Errorf("decode bulk response failed: %w", err)
	}

	result := &BulkSeedResult{
		Index:     IndexName,
		Total:    len(bulkResp.Items),
		HasError: bulkResp.Errors,
		Errors:   []BulkItemError{},
	}

	if bulkResp.Errors {
		for _, item := range bulkResp.Items {
			for operation, operationResult := range item {
				if operationResult.Status >= 300 {
					result.Errors = append(result.Errors, BulkItemError{
						Operation: operation,
						ID:        operationResult.ID,
						Status:    operationResult.Status,
						Error:     operationResult.Error,
					})
				}
			}
		}
	}

	return result, nil
}

// Bulk API có thể HTTP 200 nhưng một vài item lỗi.
// Vì vậy không chỉ check res.IsError().
// Phải check thêm field errors trong response body.