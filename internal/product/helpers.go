package product

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
)

func jsonBody(v any) (*bytes.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
}

func readESError(action string, body io.Reader) error {
	data, _ := io.ReadAll(body)
	return fmt.Errorf("%s returned elasticsearch error: %s", action, string(data))
}

func parseIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func parseRequiredIntQuery(c *gin.Context, key string) (int, error) {
	value := c.Query(key)
	if value == "" {
		return 0, errors.New(key + " is required")
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func parseOptionalFloat(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}

	return *value
}