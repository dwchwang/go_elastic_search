package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m *Module) DecreaseStock(c *gin.Context) {
	id := c.Param("id")

	var req DecreaseStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid decrease stock payload",
			"details": err.Error(),
		})
		return
	}

	body, err := jsonBody(map[string]any{
		"script": map[string]any{
			"lang": "painless",
			"source": `
				if (ctx._source.stock == null) {
					ctx.op = 'noop';
				} else if (ctx._source.stock < params.quantity) {
					ctx.op = 'noop';
				} else {
					ctx._source.stock -= params.quantity;
				}
			`,
			"params": map[string]any{
				"quantity": req.Quantity,
			},
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode script body failed"})
		return
	}

	res, err := m.es.Update(
		IndexName,
		id,
		body,
		m.es.Update.WithRefresh("true"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("decrease stock failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("decrease stock", res.Body).Error(),
		})
		return
	}

	result, err := decodeProductUpdateResult(res.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Result == "noop" {
		c.JSON(http.StatusConflict, gin.H{
			"message": "stock is not enough or stock field is missing",
			"result":  result,
		})
		return
	}

	updatedProduct, err := m.getProductDoc(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "stock decreased",
			"result":  result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "stock decreased",
		"result":  result,
		"data":    updatedProduct,
	})
}

func (m *Module) IncreaseViewCount(c *gin.Context) {
	id := c.Param("id")

	body, err := jsonBody(map[string]any{
		"script": map[string]any{
			"lang": "painless",
			"source": `
				if (ctx._source.view_count == null) {
					ctx._source.view_count = 1;
				} else {
					ctx._source.view_count += 1;
				}
			`,
		},
		"upsert": Product{
			ID:          id,
			Name:        "Unknown product",
			Description: "Created by view upsert demo",
			Brand:       "Unknown",
			Category:    "unknown",
			Price:       0,
			Stock:       0,
			Rating:      0,
			ViewCount:   1,
			Tags:        []string{},
			Specs:       []ProductSpec{},
			CreatedAt:   "2025-01-01",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode upsert body failed"})
		return
	}

	res, err := m.es.Update(
		IndexName,
		id,
		body,
		m.es.Update.WithRefresh("true"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("increase view count failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("increase view count", res.Body).Error(),
		})
		return
	}

	result, err := decodeProductUpdateResult(res.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	productDoc, err := m.getProductDoc(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "view count increased",
			"result":  result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "view count increased",
		"result":  result,
		"data":    productDoc,
	})
}

func (m *Module) UpdateProductWithConcurrency(c *gin.Context) {
	id := c.Param("id")

	seqNo, err := parseRequiredIntQuery(c, "seq_no")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "seq_no query param is required and must be a valid integer",
		})
		return
	}

	primaryTerm, err := parseRequiredIntQuery(c, "primary_term")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "primary_term query param is required and must be a valid integer",
		})
		return
	}

	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid update payload",
			"details": err.Error(),
		})
		return
	}

	delete(patch, "id")

	if len(patch) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "update payload is empty",
		})
		return
	}

	body, err := jsonBody(map[string]any{
		"doc": patch,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode concurrent update body failed"})
		return
	}

	res, err := m.es.Update(
		IndexName,
		id,
		body,
		m.es.Update.WithIfSeqNo(seqNo),
		m.es.Update.WithIfPrimaryTerm(primaryTerm),
		m.es.Update.WithRefresh("true"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("concurrent update failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	if res.StatusCode == http.StatusConflict {
		c.JSON(http.StatusConflict, gin.H{
			"error": "version conflict",
			"message": "product was changed by another request. Get the latest product and retry with the new seq_no and primary_term.",
		})
		return
	}

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("concurrent update", res.Body).Error(),
		})
		return
	}

	result, err := decodeProductUpdateResult(res.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedProduct, err := m.getProductDoc(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "product updated with concurrency control",
			"result":  result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "product updated with concurrency control",
		"result":  result,
		"data":    updatedProduct,
	})
}

func decodeProductUpdateResult(body io.Reader) (*ProductUpdateResult, error) {
	var esResp struct {
		ID          string `json:"_id"`
		Result      string `json:"result"`
		SeqNo       int64  `json:"_seq_no"`
		PrimaryTerm int64  `json:"_primary_term"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode update response failed: %w", err)
	}

	return &ProductUpdateResult{
		ID:          esResp.ID,
		Result:      esResp.Result,
		SeqNo:       esResp.SeqNo,
		PrimaryTerm: esResp.PrimaryTerm,
	}, nil
}


// DecreaseStock:
// - dùng script
// - ctx._source.stock -= params.quantity
// - nếu stock không đủ thì ctx.op = noop

// IncreaseViewCount:
// - document tồn tại thì tăng view_count
// - document chưa tồn tại thì insert bằng upsert

// UpdateProductWithConcurrency:
// - dùng if_seq_no + if_primary_term
// - nếu version cũ thì Elasticsearch trả 409 conflict