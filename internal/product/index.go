package product

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (m *Module) SetupIndex(c *gin.Context) {
	if err := m.createIndexIfNotExists(IndexName, productsMapping); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "index created successfully",
		"index":   IndexName,
	})
}

func (m *Module) createIndexIfNotExists(indexName string, mapping string) error {
	existsRes, err := m.es.Indices.Exists([]string{indexName})
	if err != nil {
		return fmt.Errorf("check index exists failed: %w", err)
	}
	defer existsRes.Body.Close()

	if existsRes.StatusCode == http.StatusOK {
		log.Printf("index %s already exists", indexName)
		return nil
	}

	if existsRes.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(existsRes.Body)
		return fmt.Errorf("check index exists returned unexpected status %d: %s", existsRes.StatusCode, string(body))
	}

	createRes, err := m.es.Indices.Create(
		indexName,
		m.es.Indices.Create.WithBody(strings.NewReader(mapping)),
	)
	if err != nil {
		return fmt.Errorf("create index %s failed: %w", indexName, err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		body, _ := io.ReadAll(createRes.Body)
		return fmt.Errorf("create index %s returned error: %s", indexName, string(body))
	}

	log.Printf("created index %s", indexName)
	return nil
}

const productsMapping = `
{
  "settings": {
    "analysis": {
      "filter": {
        "product_synonyms": {
          "type": "synonym_graph",
          "synonyms": [
            "laptop, notebook",
            "phone, smartphone, cellphone",
            "earbuds, earphones",
            "tv, television"
          ]
        }
      },
      "analyzer": {
        "product_search_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": [
            "lowercase",
            "product_synonyms"
          ]
        },
        "english_text_analyzer": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": [
            "lowercase",
            "stop",
            "porter_stem"
          ]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id": {
        "type": "keyword"
      },
      "name": {
        "type": "text",
        "analyzer": "standard",
        "search_analyzer": "product_search_analyzer",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          }
        }
      },
      "description": {
        "type": "text",
        "analyzer": "english_text_analyzer"
      },
      "brand": {
        "type": "keyword"
      },
      "category": {
        "type": "keyword"
      },
      "price": {
        "type": "double"
      },
      "stock": {
        "type": "integer"
      },
      "rating": {
        "type": "float"
      },
      "view_count": {
        "type": "integer"
      },
      "tags": {
        "type": "keyword"
      },
      "specs": {
        "type": "nested",
        "properties": {
          "name": {
            "type": "keyword"
          },
          "value": {
            "type": "keyword"
          }
        }
      },
      "created_at": {
        "type": "date"
      }
    }
  }
}
`


// name là text để full-text search.
// name.keyword là keyword để sort/exact/aggregation nếu cần.
// brand/category/tags là keyword vì dùng filter và aggregation.
// specs là nested để query đúng cặp name/value.
// description dùng analyzer có stop + stemmer.
// name search_analyzer có synonym: laptop/notebook, phone/smartphone.