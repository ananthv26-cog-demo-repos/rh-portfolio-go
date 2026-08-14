package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/domain"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/store"
)

func TestExponentParityBoundaries(t *testing.T) {
	price, _ := money.Parse("19.990000")
	handler := NewHandler(&store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct-p1\x00HOOD": {{Quantity: 10, Price: price}},
	}})
	tests := []struct {
		name     string
		mark     string
		status   int
		bodyPart string
	}{
		{"precision limit", "1e25", http.StatusOK,
			`"unrealized_pnl":"99999999999999999999999800.10"`},
		{"precision exceeded", "1e26", http.StatusInternalServerError, ""},
		{"larger exponent", "1e400", http.StatusInternalServerError, ""},
		{"fractional precision exceeded", "1e25.5", http.StatusInternalServerError, ""},
		{"tiny exponent", "1e-1000000000", http.StatusOK,
			`"unrealized_pnl":"-199.90"`},
		{"tiny exponent bounded", "1e-400", http.StatusOK,
			`"unrealized_pnl":"-199.90"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet,
				"/v1/portfolio/acct-p1/HOOD/?mark="+test.mark, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if test.bodyPart != "" && !strings.Contains(response.Body.String(), test.bodyPart) {
				t.Fatalf("body = %q, want substring %q", response.Body.String(), test.bodyPart)
			}
		})
	}
}

func TestUnknownPositionPrecedesInvalidMark(t *testing.T) {
	handler := NewHandler(&store.MemoryStore{Positions: map[string][]domain.Lot{}})
	request := httptest.NewRequest(http.MethodGet,
		"/v1/portfolio/unknown/NOPE/?mark=not-a-number", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestMarkPresenceDistinguishesAbsentAndEmpty(t *testing.T) {
	price, _ := money.Parse("19.990000")
	handler := NewHandler(&store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct-p1\x00HOOD": {{Quantity: 10, Price: price}},
	}})
	tests := []struct {
		name   string
		query  string
		status int
	}{
		{"absent defaults to zero", "", http.StatusOK},
		{"present but empty", "?mark=", http.StatusInternalServerError},
		{"bare parameter", "?mark", http.StatusInternalServerError},
		{"invalid value", "?mark=abc", http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet,
				"/v1/portfolio/acct-p1/HOOD/"+test.query, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
