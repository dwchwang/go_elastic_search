package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ProductAnalyticsResult struct {
	TotalProducts      int64                   `json:"total_products"`
	UniqueBrands       float64                 `json:"unique_brands"`
	PriceStats         ProductPriceStats       `json:"price_stats"`
	RatingPercentiles  map[string]float64      `json:"rating_percentiles"`
	ProductsByCategory []ProductCategoryBucket `json:"products_by_category"`
	TopBrands          []ProductBrandBucket    `json:"top_brands"`
	PriceRanges        []ProductPriceRange     `json:"price_ranges"`
	TopTags            []ProductTagBucket      `json:"top_tags"`
}

type ProductPriceStats struct {
	Count int64   `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	Sum   float64 `json:"sum"`
}

type ProductCategoryBucket struct {
	Category      string  `json:"category"`
	Count         int64   `json:"count"`
	AveragePrice  float64 `json:"average_price"`
	MinPrice      float64 `json:"min_price"`
	MaxPrice      float64 `json:"max_price"`
	AverageRating float64 `json:"average_rating"`
	TotalStock    float64 `json:"total_stock"`
}

type ProductBrandBucket struct {
	Brand string `json:"brand"`
	Count int64  `json:"count"`
}

type ProductPriceRange struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

type ProductTagBucket struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

func (m *Module) GetProductAnalytics(c *gin.Context) {
	result, err := m.getProductAnalytics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (m *Module) getProductAnalytics() (*ProductAnalyticsResult, error) {
	query := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"unique_brands": map[string]any{
				"cardinality": map[string]any{
					"field": "brand",
				},
			},
			"price_stats": map[string]any{
				"stats": map[string]any{
					"field": "price",
				},
			},
			"rating_percentiles": map[string]any{
				"percentiles": map[string]any{
					"field":    "rating",
					"percents": []int{25, 50, 75, 95},
				},
			},
			"products_by_category": map[string]any{
				"terms": map[string]any{
					"field": "category",
					"size":  10,
				},
				"aggs": map[string]any{
					"average_price": map[string]any{
						"avg": map[string]any{
							"field": "price",
						},
					},
					"min_price": map[string]any{
						"min": map[string]any{
							"field": "price",
						},
					},
					"max_price": map[string]any{
						"max": map[string]any{
							"field": "price",
						},
					},
					"average_rating": map[string]any{
						"avg": map[string]any{
							"field": "rating",
						},
					},
					"total_stock": map[string]any{
						"sum": map[string]any{
							"field": "stock",
						},
					},
				},
			},
			"top_brands": map[string]any{
				"terms": map[string]any{
					"field": "brand",
					"size":  10,
				},
			},
			"price_ranges": map[string]any{
				"range": map[string]any{
					"field": "price",
					"ranges": []map[string]any{
						{
							"key": "under_5m",
							"to":  5000000,
						},
						{
							"key":  "5m_to_15m",
							"from": 5000000,
							"to":   15000000,
						},
						{
							"key":  "15m_to_30m",
							"from": 15000000,
							"to":   30000000,
						},
						{
							"key":  "above_30m",
							"from": 30000000,
						},
					},
				},
			},
			"top_tags": map[string]any{
				"terms": map[string]any{
					"field": "tags",
					"size":  10,
				},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal product analytics query failed: %w", err)
	}

	res, err := m.es.Search(
		m.es.Search.WithIndex(IndexName),
		m.es.Search.WithBody(bytes.NewReader(body)),
		m.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("product analytics search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, readESError("product analytics", res.Body)
	}

	return decodeProductAnalyticsResponse(res.Body)
}

func decodeProductAnalyticsResponse(body io.Reader) (*ProductAnalyticsResult, error) {
	var esResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			UniqueBrands struct {
				Value float64 `json:"value"`
			} `json:"unique_brands"`

			PriceStats struct {
				Count int64    `json:"count"`
				Min   *float64 `json:"min"`
				Max   *float64 `json:"max"`
				Avg   *float64 `json:"avg"`
				Sum   *float64 `json:"sum"`
			} `json:"price_stats"`

			RatingPercentiles struct {
				Values map[string]float64 `json:"values"`
			} `json:"rating_percentiles"`

			ProductsByCategory struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64 `json:"doc_count"`

					AveragePrice struct {
						Value *float64 `json:"value"`
					} `json:"average_price"`

					MinPrice struct {
						Value *float64 `json:"value"`
					} `json:"min_price"`

					MaxPrice struct {
						Value *float64 `json:"value"`
					} `json:"max_price"`

					AverageRating struct {
						Value *float64 `json:"value"`
					} `json:"average_rating"`

					TotalStock struct {
						Value *float64 `json:"value"`
					} `json:"total_stock"`
				} `json:"buckets"`
			} `json:"products_by_category"`

			TopBrands struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"top_brands"`

			PriceRanges struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"price_ranges"`

			TopTags struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64 `json:"doc_count"`
				} `json:"buckets"`
			} `json:"top_tags"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode product analytics response failed: %w", err)
	}

	categoryBuckets := make([]ProductCategoryBucket, 0, len(esResp.Aggregations.ProductsByCategory.Buckets))
	for _, bucket := range esResp.Aggregations.ProductsByCategory.Buckets {
		categoryBuckets = append(categoryBuckets, ProductCategoryBucket{
			Category:      bucket.Key,
			Count:         bucket.DocCount,
			AveragePrice:  valueOrZero(bucket.AveragePrice.Value),
			MinPrice:      valueOrZero(bucket.MinPrice.Value),
			MaxPrice:      valueOrZero(bucket.MaxPrice.Value),
			AverageRating: valueOrZero(bucket.AverageRating.Value),
			TotalStock:    valueOrZero(bucket.TotalStock.Value),
		})
	}

	brandBuckets := make([]ProductBrandBucket, 0, len(esResp.Aggregations.TopBrands.Buckets))
	for _, bucket := range esResp.Aggregations.TopBrands.Buckets {
		brandBuckets = append(brandBuckets, ProductBrandBucket{
			Brand: bucket.Key,
			Count: bucket.DocCount,
		})
	}

	priceRanges := make([]ProductPriceRange, 0, len(esResp.Aggregations.PriceRanges.Buckets))
	for _, bucket := range esResp.Aggregations.PriceRanges.Buckets {
		priceRanges = append(priceRanges, ProductPriceRange{
			Range: bucket.Key,
			Count: bucket.DocCount,
		})
	}

	tagBuckets := make([]ProductTagBucket, 0, len(esResp.Aggregations.TopTags.Buckets))
	for _, bucket := range esResp.Aggregations.TopTags.Buckets {
		tagBuckets = append(tagBuckets, ProductTagBucket{
			Tag:   bucket.Key,
			Count: bucket.DocCount,
		})
	}

	return &ProductAnalyticsResult{
		TotalProducts: esResp.Hits.Total.Value,
		UniqueBrands:  esResp.Aggregations.UniqueBrands.Value,
		PriceStats: ProductPriceStats{
			Count: esResp.Aggregations.PriceStats.Count,
			Min:   valueOrZero(esResp.Aggregations.PriceStats.Min),
			Max:   valueOrZero(esResp.Aggregations.PriceStats.Max),
			Avg:   valueOrZero(esResp.Aggregations.PriceStats.Avg),
			Sum:   valueOrZero(esResp.Aggregations.PriceStats.Sum),
		},
		RatingPercentiles:  esResp.Aggregations.RatingPercentiles.Values,
		ProductsByCategory: categoryBuckets,
		TopBrands:          brandBuckets,
		PriceRanges:        priceRanges,
		TopTags:            tagBuckets,
	}, nil
}

// size: 0             → chỉ lấy aggregation
// stats               → min/max/avg/sum/count
// cardinality         → số brand khác nhau
// percentiles         → phân phối rating
// terms category      → group by category
// range price         → chia khoảng giá
// terms + avg/sum     → bucket + metric aggregation