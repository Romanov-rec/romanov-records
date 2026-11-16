// Package botsync связывает сайт с Telegram-ботом студии:
// заявки сайта попадают в «Расписание» бота, а сайт не даёт выбрать время,
// уже занятое бронью из бота или сеансом, поставленным вручную.
package botsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"romanov/backend/internal/core/domain"
	"strings"
	"time"
)

const secretHeader = "X-Integration-Secret"

type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

// New возвращает nil, если связка не настроена — тогда сайт работает как раньше.
func New(baseURL, secret string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	secret = strings.TrimSpace(secret)
	if baseURL == "" || secret == "" {
		return nil
	}
	return &Client{baseURL: baseURL, secret: secret, http: &http.Client{Timeout: 8 * time.Second}}
}

type bookingPayload struct {
	ID            int    `json:"id"`
	FullName      string `json:"full_name"`
	Phone         string `json:"phone"`
	Telegram      string `json:"telegram,omitempty"`
	Date          string `json:"date"`
	Time          string `json:"time"`
	DurationHours int    `json:"duration_hours"`
	Details       string `json:"details"`
	Comment       string `json:"comment,omitempty"`
	Status        string `json:"status"`
}

// PushBooking создаёт или обновляет заявку в расписании бота (повторный вызов безопасен).
func (c *Client) PushBooking(ctx context.Context, b domain.Booking) error {
	if c == nil {
		return nil
	}
	if b.ID <= 0 || b.DesiredDate.Year() <= 1970 {
		return nil
	}
	body, err := json.Marshal(bookingPayload{
		ID:            b.ID,
		FullName:      b.FullName,
		Phone:         b.PhoneNumber,
		Telegram:      b.TelegramUsername,
		Date:          b.DesiredDate.Format("2006-01-02"),
		Time:          b.DesiredTime.Format("15:04"),
		DurationHours: max(b.DurationHours, 1),
		Details:       b.RequestDetails,
		Comment:       b.Comment,
		Status:        string(b.Status),
	})
	if err != nil {
		return fmt.Errorf("marshal booking: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/integrations/site/bookings", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(secretHeader, c.secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post booking: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bot answered %d", resp.StatusCode)
	}
	return nil
}

// BusyTimes — занятые в боте получасы дня в формате «ЧЧ:ММ».
func (c *Client) BusyTimes(ctx context.Context, date string) ([]string, error) {
	if c == nil {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/integrations/site/busy?date="+url.QueryEscape(date), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set(secretHeader, c.secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get busy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bot answered %d", resp.StatusCode)
	}
	var out struct {
		Busy []string `json:"busy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode busy: %w", err)
	}
	return out.Busy, nil
}

// Backfill отправляет в бота все актуальные заявки — при запуске сайта,
// чтобы подтянуть те, что были созданы до настройки связки или пока бот был недоступен.
func (c *Client) Backfill(ctx context.Context, bookings []domain.Booking) {
	if c == nil {
		return
	}
	from := time.Now().AddDate(0, 0, -1)
	sent, failed := 0, 0
	for _, b := range bookings {
		if b.Status == domain.BookingStatusCancelled || b.DesiredDate.Before(from) {
			continue
		}
		if err := c.PushBooking(ctx, b); err != nil {
			failed++
			slog.Warn("bot sync backfill failed", "booking_id", b.ID, "error", err)
			continue
		}
		sent++
	}
	slog.Info("bot sync backfill done", "sent", sent, "failed", failed)
}
