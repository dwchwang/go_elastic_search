package esclient

import (
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

func New() (*elasticsearch.Client, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create elasticsearch client failed: %w", err)
	}

	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("connect to elasticsearch failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch info API returned error: %s", res.String())
	}

	return client, nil
}

// File này chỉ có một nhiệm vụ:
// Tạo official Elasticsearch client và check Elasticsearch có chạy không.