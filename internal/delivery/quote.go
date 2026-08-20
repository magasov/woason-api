package delivery

import (
	"fmt"
	"math"
	"strings"
)

const (
	MethodCDEK   = "cdek"
	MethodPochta = "pochta"
	MethodPickup = "pickup"
)

var cityFactor = map[string]float64{
	"москва":          1.0,
	"санкт-петербург": 1.15,
	"казань":          1.25,
	"новосибирск":     1.55,
	"краснодар":       1.35,
	"екатеринбург":    1.4,
}

type Quote struct {
	Price int    `json:"price"`
	ETA   string `json:"eta"`
	Days  string `json:"days"`
}

func Factor(city string) float64 {
	if f, ok := cityFactor[strings.ToLower(strings.TrimSpace(city))]; ok {
		return f
	}
	return 1.4
}

func QuoteDelivery(method, city string, weightKg float64) (Quote, error) {
	switch method {
	case MethodCDEK, MethodPochta, MethodPickup:
	default:
		return Quote{}, fmt.Errorf("неизвестный способ доставки")
	}
	weight := math.Max(0.3, weightKg)
	factor := Factor(city)

	if method == MethodPickup {
		return Quote{Price: 0, ETA: "Сегодня — завтра", Days: "встреча с продавцом"}, nil
	}
	if method == MethodCDEK {
		price := int(math.Round(290 + factor*weight*85))
		from := int(math.Max(1, math.Round(factor)))
		to := from + 2
		return Quote{
			Price: price,
			ETA:   fmt.Sprintf("%d–%d дня", from, to),
			Days:  "склад СДЭК / курьер",
		}, nil
	}
	price := int(math.Round(190 + factor*weight*55))
	from := 5 + int(math.Round((factor-1)*4))
	to := from + 7
	return Quote{
		Price: price,
		ETA:   fmt.Sprintf("%d–%d дней", from, to),
		Days:  "отделение Почты России",
	}, nil
}

func MakeTrackNumber(method string, n int64) string {
	if n < 100000000 {
		n += 100000000
	}
	switch method {
	case MethodCDEK:
		return fmt.Sprintf("CDEK%d", n)
	case MethodPochta:
		s := fmt.Sprintf("%d", n)
		if len(s) > 12 {
			s = s[:12]
		}
		return "14" + s + "RU"
	default:
		return fmt.Sprintf("PICKUP-%d", n)
	}
}
