package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ProductSearchParams struct {
	Query    string
	Category string
	Brand    string
	MinPrice *float64
	MaxPrice *float64
	Sort     string
	Page     int
	Size     int
}

type ProductSearchResult struct {
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
	Items []ProductSearchHit `json:"items"`
}

type ProductSearchHit struct {
	ID        string              `json:"id"`
	Score     *float64            `json:"score,omitempty"`
	Product   Product             `json:"product"`
	Highlight map[string][]string `json:"highlight,omitempty"`
}

type ProductSpecSearchResult struct {
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
	Items []ProductSpecSearchHit `json:"items"`
}

type ProductSpecSearchHit struct {
	ID                 string        `json:"id"`
	Score              *float64      `json:"score,omitempty"`
	Product            Product       `json:"product"`
	MatchedNestedSpecs []ProductSpec `json:"matched_nested_specs,omitempty"`
}

func (m *Module) SearchProducts(c *gin.Context) {
	page, err := parseIntQuery(c, "page", 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a valid integer"})
		return
	}

	size, err := parseIntQuery(c, "size", 10)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "size must be a valid integer"})
		return
	}

	minPrice, err := parseOptionalFloat(c.Query("min_price"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_price must be a valid number"})
		return
	}

	maxPrice, err := parseOptionalFloat(c.Query("max_price"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_price must be a valid number"})
		return
	}

	params := ProductSearchParams{
		Query:    c.Query("q"),
		Category: c.Query("category"),
		Brand:    c.Query("brand"),
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		Sort:     c.Query("sort"),
		Page:     page,
		Size:     size,
	}

	result, err := m.searchProducts(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (m *Module) searchProducts(params ProductSearchParams) (*ProductSearchResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.Size <= 0 {
		params.Size = 10
	}

	if params.Size > 50 {
		params.Size = 50
	}

	from := (params.Page - 1) * params.Size
	queryBody := buildProductSearchQuery(params, from)

	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("marshal search query failed: %w", err)
	}

	res, err := m.es.Search(
		m.es.Search.WithIndex(IndexName),
		m.es.Search.WithBody(bytes.NewReader(body)),
		m.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("search products failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, readESError("search products", res.Body)
	}

	return decodeProductSearchResponse(res.Body, params.Page, params.Size)
}

func buildProductSearchQuery(params ProductSearchParams, from int) map[string]any {
	mustQueries := []map[string]any{}
	filterQueries := []map[string]any{}

	if params.Query != "" {
		mustQueries = append(mustQueries, map[string]any{
			"multi_match": map[string]any{
				"query":     params.Query,
				"fields":    []string{"name^3", "description"},
				"fuzziness": "AUTO",
			},
		})
	}

	if params.Category != "" {
		filterQueries = append(filterQueries, map[string]any{
			"term": map[string]any{
				"category": params.Category,
			},
		})
	}

	if params.Brand != "" {
		filterQueries = append(filterQueries, map[string]any{
			"term": map[string]any{
				"brand": params.Brand,
			},
		})
	}

	priceRange := map[string]any{}

	if params.MinPrice != nil {
		priceRange["gte"] = *params.MinPrice
	}

	if params.MaxPrice != nil {
		priceRange["lte"] = *params.MaxPrice
	}

	if len(priceRange) > 0 {
		filterQueries = append(filterQueries, map[string]any{
			"range": map[string]any{
				"price": priceRange,
			},
		})
	}

	var query map[string]any

	if len(mustQueries) == 0 && len(filterQueries) == 0 {
		query = map[string]any{
			"match_all": map[string]any{},
		}
	} else {
		boolQuery := map[string]any{}

		if len(mustQueries) > 0 {
			boolQuery["must"] = mustQueries
		}

		if len(filterQueries) > 0 {
			boolQuery["filter"] = filterQueries
		}

		query = map[string]any{
			"bool": boolQuery,
		}
	}

	searchBody := map[string]any{
		"from":  from,
		"size":  params.Size,
		"query": query,
		"highlight": map[string]any{
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
			"fields": map[string]any{
				"name":        map[string]any{},
				"description": map[string]any{},
			},
		},
	}

	sort := buildProductSort(params.Sort)
	if len(sort) > 0 {
		searchBody["sort"] = sort
	}

	return searchBody
}

func buildProductSort(sort string) []map[string]any {
	switch sort {
	case "price_asc":
		return []map[string]any{
			{"price": "asc"},
		}
	case "price_desc":
		return []map[string]any{
			{"price": "desc"},
		}
	case "rating_desc":
		return []map[string]any{
			{"rating": "desc"},
		}
	case "newest":
		return []map[string]any{
			{"created_at": "desc"},
		}
	default:
		return nil
	}
}

func decodeProductSearchResponse(body io.Reader, page int, size int) (*ProductSearchResult, error) {
	var esResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string              `json:"_id"`
				Score     *float64            `json:"_score"`
				Source    Product             `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode product search response failed: %w", err)
	}

	items := make([]ProductSearchHit, 0, len(esResp.Hits.Hits))

	for _, hit := range esResp.Hits.Hits {
		items = append(items, ProductSearchHit{
			ID:        hit.ID,
			Score:     hit.Score,
			Product:   hit.Source,
			Highlight: hit.Highlight,
		})
	}

	return &ProductSearchResult{
		Total: esResp.Hits.Total.Value,
		Page:  page,
		Size:  size,
		Items: items,
	}, nil
}

func (m *Module) SearchProductsBySpec(c *gin.Context) {
	name := c.Query("name")
	value := c.Query("value")

	if name == "" || value == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "name and value query params are required",
			"example": "/search/products/specs?name=storage&value=256GB",
		})
		return
	}

	page, err := parseIntQuery(c, "page", 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a valid integer"})
		return
	}

	size, err := parseIntQuery(c, "size", 10)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "size must be a valid integer"})
		return
	}

	result, err := m.searchProductsBySpec(name, value, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (m *Module) searchProductsBySpec(name string, value string, page int, size int) (*ProductSpecSearchResult, error) {
	if page <= 0 {
		page = 1
	}

	if size <= 0 {
		size = 10
	}

	if size > 50 {
		size = 50
	}

	from := (page - 1) * size

	queryBody := map[string]any{
		"from": from,
		"size": size,
		"query": map[string]any{
			"nested": map[string]any{
				"path": "specs",
				"query": map[string]any{
					"bool": map[string]any{
						"must": []map[string]any{
							{
								"term": map[string]any{
									"specs.name": name,
								},
							},
							{
								"term": map[string]any{
									"specs.value": value,
								},
							},
						},
					},
				},
				"inner_hits": map[string]any{},
			},
		},
	}

	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("marshal nested spec query failed: %w", err)
	}

	res, err := m.es.Search(
		m.es.Search.WithIndex(IndexName),
		m.es.Search.WithBody(bytes.NewReader(body)),
		m.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("search products by spec failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, readESError("search products by spec", res.Body)
	}

	return decodeProductSpecSearchResponse(res.Body, page, size)
}

func decodeProductSpecSearchResponse(body io.Reader, page int, size int) (*ProductSpecSearchResult, error) {
	var esResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string   `json:"_id"`
				Score     *float64 `json:"_score"`
				Source    Product  `json:"_source"`
				InnerHits struct {
					Specs struct {
						Hits struct {
							Hits []struct {
								Source ProductSpec `json:"_source"`
							} `json:"hits"`
						} `json:"hits"`
					} `json:"specs"`
				} `json:"inner_hits"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode nested spec search response failed: %w", err)
	}

	items := make([]ProductSpecSearchHit, 0, len(esResp.Hits.Hits))

	for _, hit := range esResp.Hits.Hits {
		matchedSpecs := make([]ProductSpec, 0, len(hit.InnerHits.Specs.Hits.Hits))

		for _, innerHit := range hit.InnerHits.Specs.Hits.Hits {
			matchedSpecs = append(matchedSpecs, innerHit.Source)
		}

		items = append(items, ProductSpecSearchHit{
			ID:                 hit.ID,
			Score:              hit.Score,
			Product:            hit.Source,
			MatchedNestedSpecs: matchedSpecs,
		})
	}

	return &ProductSpecSearchResult{
		Total: esResp.Hits.Total.Value,
		Page:  page,
		Size:  size,
		Items: items,
	}, nil
}

// q              → multi_match, fuzzy, highlight
// category/brand → term filter
// min/max price  → range filter
// sort           → field sort
// page/size      → from + size pagination
// specs          → nested query + inner_hits
