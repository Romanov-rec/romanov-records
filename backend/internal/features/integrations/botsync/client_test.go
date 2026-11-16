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
