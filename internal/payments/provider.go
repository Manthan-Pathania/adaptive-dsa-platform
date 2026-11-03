package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// RazorpayProvider implements PaymentProvider using Razorpay's official REST API v1 and HMAC-SHA256 signatures.
type RazorpayProvider struct {
	keyID         string
	keySecret     string
	webhookSecret string
	httpClient    *http.Client
	baseURL       string
}

func NewRazorpayProvider(keyID, keySecret, webhookSecret string, timeout time.Duration) *RazorpayProvider {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &RazorpayProvider{
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: "https://api.razorpay.com/v1",
	}
}

func (r *RazorpayProvider) KeyID() string {
	return r.keyID
}

func (r *RazorpayProvider) CreateOrder(ctx context.Context, req CreateProviderOrderRequest) (*ProviderOrderResponse, error) {
	if r.keyID == "" || r.keySecret == "" {
		return nil, fmt.Errorf("razorpay: api credentials not configured")
	}

	payload := map[string]interface{}{
		"amount":   req.AmountPaise,
		"currency": req.Currency,
		"receipt":  req.Receipt,
		"notes":    req.Notes,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("razorpay: marshal order payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/orders", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("razorpay: create order request: %w", err)
	}

	httpReq.SetBasicAuth(r.keyID, r.keySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("razorpay: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("%w: status %d (%s)", ErrProviderUnavailable, resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		ID       string `json:"id"`
		Amount   int32  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}

	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("razorpay: unmarshal order response: %w", err)
	}

	return &ProviderOrderResponse{
		ID:       res.ID,
		Amount:   res.Amount,
		Currency: res.Currency,
		Status:   res.Status,
	}, nil
}

// GetPayment fetches authoritative payment status directly from Razorpay REST API v1.
func (r *RazorpayProvider) GetPayment(ctx context.Context, paymentID string) (*PaymentDetails, error) {
	if r.keyID == "" || r.keySecret == "" {
		return nil, fmt.Errorf("razorpay: api credentials not configured")
	}

	endpoint := fmt.Sprintf("%s/payments/%s", r.baseURL, paymentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("razorpay: create get payment request: %w", err)
	}
	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("razorpay: get payment network call: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("razorpay: read payment response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay: get payment failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		ID       string `json:"id"`
		OrderID  string `json:"order_id"`
		Amount   int32  `json:"amount"`
		Currency string `json:"currency"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("razorpay: unmarshal payment response: %w", err)
	}

	return &PaymentDetails{
		ID:          res.ID,
		OrderID:     res.OrderID,
		AmountPaise: res.Amount,
		Currency:    res.Currency,
		Status:      res.Status,
	}, nil
}

// VerifyCheckoutSignature verifies Razorpay Checkout modal response signature:
// HMAC-SHA256(orderID + "|" + paymentID, keySecret) == signature
func (r *RazorpayProvider) VerifyCheckoutSignature(orderID, paymentID, signature string) bool {
	if orderID == "" || paymentID == "" || signature == "" || r.keySecret == "" {
		return false
	}

	data := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(r.keySecret))
	mac.Write([]byte(data))
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(expectedHex), []byte(signature)) == 1
}

// VerifyWebhookSignature verifies Razorpay webhook payload signature:
// HMAC-SHA256(rawBody, webhookSecret) == signature
func (r *RazorpayProvider) VerifyWebhookSignature(rawBody []byte, signature string) bool {
	if len(rawBody) == 0 || signature == "" || r.webhookSecret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(r.webhookSecret))
	mac.Write(rawBody)
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(expectedHex), []byte(signature)) == 1
}

// MockPaymentProvider provides deterministic payment gateway simulation for automated tests.
type MockPaymentProvider struct {
	keyID                 string
	keySecret             string
	webhookSecret         string
	FailOrderCreation     bool
	FailSignatureVerify   bool
	MockPaymentStatus     string
	Payments              map[string]*PaymentDetails
	RecordedOrderRequests []CreateProviderOrderRequest
}

func NewMockPaymentProvider() *MockPaymentProvider {
	return &MockPaymentProvider{
		keyID:             "rzp_test_mock_key_id",
		keySecret:         "mock_key_secret_for_unit_tests",
		webhookSecret:     "mock_webhook_secret_for_unit_tests",
		MockPaymentStatus: "captured",
		Payments:          make(map[string]*PaymentDetails),
	}
}

func (m *MockPaymentProvider) RegisterPayment(p *PaymentDetails) {
	if m.Payments == nil {
		m.Payments = make(map[string]*PaymentDetails)
	}
	m.Payments[p.ID] = p
}

func (m *MockPaymentProvider) GetPayment(_ context.Context, paymentID string) (*PaymentDetails, error) {
	if m.Payments != nil {
		if p, ok := m.Payments[paymentID]; ok {
			return p, nil
		}
	}
	status := m.MockPaymentStatus
	if status == "" {
		status = "captured"
	}
	return &PaymentDetails{
		ID:          paymentID,
		OrderID:     "",
		AmountPaise: 0,
		Currency:    "INR",
		Status:      status,
	}, nil
}

func (m *MockPaymentProvider) KeyID() string {
	return m.keyID
}

func (m *MockPaymentProvider) CreateOrder(_ context.Context, req CreateProviderOrderRequest) (*ProviderOrderResponse, error) {
	if m.FailOrderCreation {
		return nil, ErrProviderUnavailable
	}
	m.RecordedOrderRequests = append(m.RecordedOrderRequests, req)
	mockOrderID := fmt.Sprintf("order_mock_%s", uuid.New().String()[:12])
	return &ProviderOrderResponse{
		ID:       mockOrderID,
		Amount:   req.AmountPaise,
		Currency: req.Currency,
		Status:   "created",
	}, nil
}

func (m *MockPaymentProvider) ComputeCheckoutSignature(orderID, paymentID string) string {
	data := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(m.keySecret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *MockPaymentProvider) ComputeWebhookSignature(rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(m.webhookSecret))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

func (m *MockPaymentProvider) VerifyCheckoutSignature(orderID, paymentID, signature string) bool {
	if m.FailSignatureVerify {
		return false
	}
	if signature == "mock_signature_test" {
		return true
	}
	expected := m.ComputeCheckoutSignature(orderID, paymentID)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func (m *MockPaymentProvider) VerifyWebhookSignature(rawBody []byte, signature string) bool {
	if m.FailSignatureVerify {
		return false
	}
	expected := m.ComputeWebhookSignature(rawBody)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}
