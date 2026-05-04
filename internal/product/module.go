package product

import (
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
)

const IndexName = "products"

type Module struct {
	es *elasticsearch.Client
}

func NewModule(es *elasticsearch.Client) *Module {
	return &Module{
		es: es,
	}
}

func (m *Module) RegisterRoutes(router *gin.Engine) {
	router.POST("/setup", m.SetupIndex)
	router.POST("/seed", m.SeedProducts)

	router.POST("/products", m.CreateProduct)
	router.GET("/products/:id", m.GetProduct)
	router.PATCH("/products/:id", m.UpdateProduct)
	router.DELETE("/products/:id", m.DeleteProduct)

	router.GET("/search/products", m.SearchProducts)
	router.GET("/search/products/specs", m.SearchProductsBySpec)

	router.POST("/products/:id/decrease-stock", m.DecreaseStock)
	router.POST("/products/:id/view", m.IncreaseViewCount)
	router.PATCH("/products/:id/concurrent", m.UpdateProductWithConcurrency)

	router.GET("/analytics/products", m.GetProductAnalytics)
}

// Module là trung tâm của product demo.
// Mentor muốn xem API nào có thể nhìn ngay trong file này.
// Không còn ProductHandler, ProductRepository, SearchRepository nhiều tầng nữa.