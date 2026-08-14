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
		if redirectPath, ok := redirectPath(request.URL.EscapedPath()); ok {
			if request.URL.RawQuery != "" {
				redirectPath += "?" + request.URL.RawQuery
			}
			writer.Header().Set("Location", redirectPath)
			writer.Header().Set("X-Content-Type-Options", "nosniff")
			writer.Header().Set("Referrer-Policy", "same-origin")
			writer.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			writer.WriteHeader(http.StatusMovedPermanently)
			return
		}
		accountID, symbol, ok := parsePath(request.URL.Path)
		if !ok {
			http.NotFound(writer, request)
			return
		}
		setRoutedHeaders(writer)
		writer.Header().Set("Allow", "GET, HEAD, OPTIONS")
		switch request.Method {
		case http.MethodGet, http.MethodHead:
		case http.MethodOptions:
			writeOptions(writer)
			return
		default:
			writeMethodNotAllowed(writer, request.Method)
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
		rawMark := "0"
		if value, present := lastMarkValue(request.URL.RawQuery); present {
			rawMark = value
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
		pnl := mark.NaNString()
		if !mark.NaN {
			pnlValue, err := domain.UnrealizedPnL(lots, mark.Value)
			if err != nil {
				writeError(writer, http.StatusInternalServerError, "invalid mark")
				return
			}
			pnl, err = pnlValue.StringFixed(2)
			if err != nil {
				writeError(writer, http.StatusInternalServerError, "invalid mark")
				return
			}
		}
		response := struct {
			Symbol        string `json:"symbol"`
			Quantity      int64  `json:"quantity"`
			AverageCost   string `json:"average_cost"`
			UnrealizedPnl string `json:"unrealized_pnl"`
		}{
			Symbol:        symbol,
			Quantity:      domain.NetQuantity(lots),
			AverageCost:   "",
			UnrealizedPnl: pnl,
		}
		response.AverageCost, err = averageCost.StringFixed(4)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "invalid position")
			return
		}
		body, err := json.Marshal(response)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "response encoding error")
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodHead {
			writer.WriteHeader(http.StatusOK)
			return
		}
		_, _ = writer.Write(body)
	}
}

func writeOptions(writer http.ResponseWriter) {
	setRoutedHeaders(writer)
	writer.Header().Set("Allow", "GET, HEAD, OPTIONS")
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(`{"name":"Position","description":"","renders":["application/json","text/html"],"parses":["application/json","application/x-www-form-urlencoded","multipart/form-data"]}`))
}

func writeMethodNotAllowed(writer http.ResponseWriter, method string) {
	setRoutedHeaders(writer)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusMethodNotAllowed)
	_, _ = writer.Write([]byte("{\"detail\":\"Method \\\"" + method + "\\\" not allowed.\"}"))
}

func setRoutedHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Vary", "Accept")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Referrer-Policy", "same-origin")
}

func lastMarkValue(rawQuery string) (string, bool) {
	var value string
	present := false
	for _, pair := range strings.Split(rawQuery, "&") {
		key, rawValue := pair, ""
		if index := strings.IndexByte(pair, '='); index >= 0 {
			key, rawValue = pair[:index], pair[index+1:]
		}
		if queryUnescape(key) == "mark" {
			value = queryUnescape(rawValue)
			present = true
		}
	}
	return value, present
}

func queryUnescape(value string) string {
	var decoded strings.Builder
	decoded.Grow(len(value))
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '+':
			decoded.WriteByte(' ')
		case '%':
			if index+2 < len(value) {
				high, highOK := fromHex(value[index+1])
				low, lowOK := fromHex(value[index+2])
				if highOK && lowOK {
					decoded.WriteByte(high<<4 | low)
					index += 2
					continue
				}
			}
			decoded.WriteByte('%')
		default:
			decoded.WriteByte(value[index])
		}
	}
	return decoded.String()
}

func fromHex(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
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
