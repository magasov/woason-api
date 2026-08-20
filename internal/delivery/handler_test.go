package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuoteHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/delivery/quote?method=cdek&city=Москва&weightKg=1", nil)
	rec := httptest.NewRecorder()
	QuoteHandler(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var q Quote
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if q.Price != 375 {
		t.Fatalf("price=%d", q.Price)
	}
}
