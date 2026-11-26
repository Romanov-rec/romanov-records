package botsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"romanov/backend/internal/core/domain"
	"testing"
	"time"
)

func TestNewDisabledWithoutConfig(t *testing.T) {
	if New("", "secret") != nil || New("https://bot", "") != nil {
		t.Fatal("New() must return nil when not configured")
	}
}

func TestPushBookingAndBusyTimes(t *testing.T) {
	var got bookingPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(secretHeader) != "s3cret-s3cret-s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/integrations/site/bookings":
			_ = json.NewDecoder(r.Body).Decode(&got)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/integrations/site/busy":
			if r.URL.Query().Get("date") != "2026-12-01" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"busy":["14:00","14:30"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL+"/", "s3cret-s3cret-s3cret")
	clock, _ := time.Parse("15:04", "18:30")
	b := domain.Booking{ID: 41, FullName: "Иван", PhoneNumber: "+79161234567", DesiredDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		DesiredTime: clock, DurationHours: 2, RequestDetails: "Запись вокала", Status: domain.BookingStatusNew}
	if err := c.PushBooking(context.Background(), b); err != nil {
		t.Fatalf("PushBooking() error = %v", err)
	}
	if got.ID != 41 || got.Date != "2026-12-01" || got.Time != "18:30" || got.DurationHours != 2 || got.Status != "new" {
		t.Fatalf("payload = %#v", got)
	}

	busy, err := c.BusyTimes(context.Background(), "2026-12-01")
	if err != nil || len(busy) != 2 || busy[0] != "14:00" {
		t.Fatalf("BusyTimes() = (%v, %v)", busy, err)
	}

	if err := New(srv.URL, "wrong-secret-wrong").PushBooking(context.Background(), b); err == nil {
		t.Fatal("PushBooking() must fail on a wrong secret")
	}
}

func TestStartPaymentSendsItemsAndReadsPayURL(t *testing.T) {
	t.Parallel()

	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Integration-Secret") != "0123456789abcdef" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/integrations/site/bookings/7/payment":
			_ = json.NewDecoder(r.Body).Decode(&got)
			_, _ = w.Write([]byte(`{"status":"pending","pay_url":"https://yoomoney.ru/x","amount":5000}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/integrations/site/bookings/7/payment":
			_, _ = w.Write([]byte(`{"status":"paid","amount":5000}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "0123456789abcdef")
	yes := true
	b := domain.Booking{ID: 7, FullName: "Иван", PhoneNumber: "+79160000000",
		DesiredDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), DesiredTime: time.Date(0, 1, 1, 18, 0, 0, 0, time.UTC), DurationHours: 2}
	res, err := c.StartPayment(context.Background(), b, []PaymentItem{{ID: "vocal", Hours: 2, Engineer: &yes}}, "https://site/?payment=7")
	if err != nil || res.PayURL != "https://yoomoney.ru/x" || res.Amount != 5000 {
		t.Fatalf("StartPayment() = %#v, %v", res, err)
	}
	if got["return_url"] != "https://site/?payment=7" || got["date"] != "2026-12-01" || got["time"] != "18:00" {
		t.Fatalf("payload = %#v", got)
	}
	items, _ := got["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["engineer"] != true {
		t.Fatalf("items = %#v", got["items"])
	}
	st, err := c.PaymentStatus(context.Background(), 7)
	if err != nil || st != "paid" {
		t.Fatalf("PaymentStatus() = %q, %v", st, err)
	}
}
