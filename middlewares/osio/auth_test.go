package middlewares

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtracToken(t *testing.T) {
	tables := []struct {
		authHeader    string
		expectedToken string
	}{
		{"Bear 1111", "1111"},
		{"1111", "1111"},
	}

	for _, table := range tables {
		actualToken, err := extractToken(table.authHeader)
		assert.Nil(t, err)
		if actualToken != table.expectedToken {
			t.Errorf("Incorrect token, want:%s, got:%s", table.expectedToken, actualToken)
		}
	}
}
