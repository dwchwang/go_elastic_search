package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m *Module) CreateProduct(c *gin.Context) {
	var product Product

	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid product payload",
			"details": err.Error(),
		})
		return
	}

	body, err := jsonBody(product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "encode product failed",
		})
		return
	}

	res, err := m.es.Index(
		IndexName,
		body,
		m.es.Index.WithDocumentID(product.ID),
		m.es.Index.WithRefresh("true"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("index product failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("index product", res.Body).Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "product created",
		"id":      product.ID,
	})
}

func (m *Module) GetProduct(c *gin.Context) {
	id := c.Param("id")

	productDoc, err := m.getProductDoc(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "product not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, productDoc)
}

func (m *Module) UpdateProduct(c *gin.Context) {
	id := c.Param("id")

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "encode update body failed",
		})
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
			"error": fmt.Sprintf("update product failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "product not found",
		})
		return
	}

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("update product", res.Body).Error(),
		})
		return
	}

	updatedProduct, err := m.getProductDoc(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "product updated",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "product updated",
		"data":    updatedProduct,
	})
}

func (m *Module) DeleteProduct(c *gin.Context) {
	id := c.Param("id")

	res, err := m.es.Delete(
		IndexName,
		id,
		m.es.Delete.WithRefresh("true"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("delete product failed: %v", err),
		})
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "product not found",
		})
		return
	}

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": readESError("delete product", res.Body).Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "product deleted",
		"id":      id,
	})
}

func (m *Module) getProductDoc(ctx context.Context, id string) (*ProductDocument, error) {
	res, err := m.es.Get(
		IndexName,
		id,
		m.es.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("get product failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if res.IsError() {
		return nil, readESError("get product", res.Body)
	}

	var esDoc struct {
		ElasticID   string  `json:"_id"`
		SeqNo       int64   `json:"_seq_no"`
		PrimaryTerm int64   `json:"_primary_term"`
		Found       bool    `json:"found"`
		Source      Product `json:"_source"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esDoc); err != nil {
		return nil, fmt.Errorf("decode get product response failed: %w", err)
	}

	if !esDoc.Found {
		return nil, ErrNotFound
	}

	return &ProductDocument{
		ElasticID:   esDoc.ElasticID,
		SeqNo:       esDoc.SeqNo,
		PrimaryTerm: esDoc.PrimaryTerm,
		Product:     esDoc.Source,
	}, nil
}

// CreateProduct → Index API
// GetProduct    → Get API, trả thêm _seq_no và _primary_term
// UpdateProduct → Update API với body {"doc": {...}}
// DeleteProduct → Delete API