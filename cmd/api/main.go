package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"elastic-product-demo/internal/esclient"
	"elastic-product-demo/internal/product"
)

func main() {
	es, err := esclient.New()
	if err != nil {
		log.Fatalf("cannot connect to elasticsearch: %v", err)
	}

	productModule := product.NewModule(es)

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":        "ok",
			"elasticsearch": "connected",
		})
	})

	productModule.RegisterRoutes(router)

	log.Println("API server is running on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}