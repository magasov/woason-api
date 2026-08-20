package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"woason-api/internal/config"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type CreateInput struct {
	OrderID     string
	AmountRub   int
	Description string
	ReturnURL   string
}

type CreateResult struct {
	ID              string
	Status          string
	ConfirmationURL string
	Raw             []byte
}

type ykPayment struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

func (c *Client) CreatePayment(in CreateInput) (*CreateResult, error) {
	body := map[string]any{
		"amount": map[string]any{
			"value":    fmt.Sprintf("%d.00", in.AmountRub),
			"currency": "RUB",
		},
		"capture": true,
		"confirmation": map[string]any{
			"type":       "redirect",
			"return_url": in.ReturnURL,
		},
		"description": in.Description,
		"metadata": map[string]any{
			"order_id": in.OrderID,
		},
	}
	rawReq, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.cfg.YKassaAPIURL+"/payments", bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.YKassaShopID, c.cfg.YKassaSecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", uuid.NewString())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("юkassa недоступна")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ошибка оплаты, попробуйте позже")
	}
	var p ykPayment
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("некорректный ответ платёжной системы")
	}
	return &CreateResult{
		ID:              p.ID,
		Status:          mapStatus(p.Status),
		ConfirmationURL: p.Confirmation.ConfirmationURL,
		Raw:             raw,
	}, nil
}

func (c *Client) Refund(paymentID string, amountRub int) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{
		"payment_id": paymentID,
		"amount": map[string]any{
			"value":    fmt.Sprintf("%d.00", amountRub),
			"currency": "RUB",
		},
	})
	req, err := http.NewRequest(http.MethodPost, c.cfg.YKassaAPIURL+"/refunds", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.YKassaShopID, c.cfg.YKassaSecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", uuid.NewString())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("юkassa недоступна")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return raw, fmt.Errorf("не удалось оформить возврат")
	}
	return raw, nil
}

type Webhook struct {
	Event  string          `json:"event"`
	Object json.RawMessage `json:"object"`
}

type PaymentObject struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Metadata struct {
		OrderID string `json:"order_id"`
	} `json:"metadata"`
}

func mapStatus(s string) string {
	switch s {
	case "waiting_for_capture":
		return "waiting_for_capture"
	case "succeeded":
		return "succeeded"
	case "canceled":
		return "canceled"
	default:
		return "pending"
	}
}
