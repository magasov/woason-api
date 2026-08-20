package delivery

import (
	"net/http"
	"strconv"

	httpx "woason-api/internal/httpx"
)

func QuoteHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	method := q.Get("method")
	city := q.Get("city")
	weight, _ := strconv.ParseFloat(q.Get("weightKg"), 64)
	quote, err := QuoteDelivery(method, city, weight)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, quote)
}
