package delivery

import "testing"

func TestQuoteMoscowCDEK(t *testing.T) {
	q, err := QuoteDelivery("cdek", "Москва", 1)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 375 { // 290 + 1.0 * 1 * 85
		t.Fatalf("price=%d", q.Price)
	}
}

func TestQuotePickupZero(t *testing.T) {
	q, err := QuoteDelivery("pickup", "Казань", 10)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 0 {
		t.Fatalf("pickup must be 0, got %d", q.Price)
	}
}

func TestQuoteMinWeightPochta(t *testing.T) {
	q, err := QuoteDelivery("pochta", "Санкт-Петербург", 0.1)
	if err != nil {
		t.Fatal(err)
	}
	// 190 + 1.15 * 0.3 * 55 = 208.975 → 209
	if q.Price != 209 {
		t.Fatalf("price=%d want 209", q.Price)
	}
}

func TestUnknownCityFactor(t *testing.T) {
	q, err := QuoteDelivery("cdek", "Тверь", 1)
	if err != nil {
		t.Fatal(err)
	}
	if q.Price != 409 { // 290 + 1.4 * 1 * 85 = 409
		t.Fatalf("price=%d", q.Price)
	}
}

func TestBadMethod(t *testing.T) {
	if _, err := QuoteDelivery("drone", "Москва", 1); err == nil {
		t.Fatal("expected error")
	}
}
