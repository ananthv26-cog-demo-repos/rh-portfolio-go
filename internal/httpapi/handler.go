package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/domain"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/store"
)

func NewHandler(positionStore store.PositionStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/v1/portfolio/", positionHandler(positionStore))
	return mux
}

func positionHandler(positionStore store.PositionStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		accountID, symbol, ok := parsePath(request.URL.Path)
		if !ok || request.Method != http.MethodGet {
			http.NotFound(writer, request)
			return
		}
		symbol = strings.ToUpper(symbol)
		lots, err := positionStore.Lots(request.Context(), accountID, symbol)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "store error")
			return
		}
		if len(lots) == 0 {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		mark, err := money.Parse(request.URL.Query().Get("mark"))
		if request.URL.Query().Get("mark") == "" {
			mark, err = money.Parse("0")
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "invalid mark")
			return
		}
		averageCost, err := domain.AverageCost(lots)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		pnl := domain.UnrealizedPnL(lots, mark)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"symbol":"%s","quantity":%d,"average_cost":"%s","unrealized_pnl":"%s"}`,
			symbol, domain.NetQuantity(lots), averageCost.StringFixed(4), pnl.StringFixed(2))
	}
}

func parsePath(path string) (string, string, bool) {
	const prefix = "/v1/portfolio/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/") {
		return "", "", false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = fmt.Fprintf(writer, `{"error":%q}`, message)
}
