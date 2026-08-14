package httpapi

import (
	"encoding/json"
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
		if redirectPath, ok := redirectPath(request.URL.Path); ok {
			if request.URL.RawQuery != "" {
				redirectPath += "?" + request.URL.RawQuery
			}
			writer.Header().Set("Location", redirectPath)
			writer.WriteHeader(http.StatusMovedPermanently)
			return
		}
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
		rawMark := request.URL.Query().Get("mark")
		if rawMark == "" {
			rawMark = "0"
		}
		mark, err := money.ParseMark(rawMark)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "invalid mark")
			return
		}
		averageCost, err := domain.AverageCost(lots)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err.Error())
			return
		}
		pnl := "NaN"
		if !mark.NaN {
			pnl = domain.UnrealizedPnL(lots, mark.Value).StringFixed(2)
		} else if mark.NegativeNaN {
			pnl = "-NaN"
		}
		response := struct {
			Symbol        string `json:"symbol"`
			Quantity      int64  `json:"quantity"`
			AverageCost   string `json:"average_cost"`
			UnrealizedPnl string `json:"unrealized_pnl"`
		}{
			Symbol:        symbol,
			Quantity:      domain.NetQuantity(lots),
			AverageCost:   averageCost.StringFixed(4),
			UnrealizedPnl: pnl,
		}
		body, err := json.Marshal(response)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "response encoding error")
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}
}

func redirectPath(path string) (string, bool) {
	const prefix = "/v1/portfolio/"
	if !strings.HasPrefix(path, prefix) || strings.HasSuffix(path, "/") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return path + "/", true
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
	body, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: message})
	_, _ = writer.Write(body)
}
