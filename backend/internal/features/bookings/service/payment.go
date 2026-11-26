package bookings_service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"romanov/backend/internal/core/domain"
	"romanov/backend/internal/features/integrations/botsync"
	"strings"
	"time"
)

// PaymentGateway — оплата через ЮKassa Telegram-бота студии.
type PaymentGateway interface {
	StartPayment(ctx context.Context, booking domain.Booking, items []botsync.PaymentItem, returnURL string) (botsync.PaymentResult, error)
	PaymentStatus(ctx context.Context, id int) (string, error)
}

// PaymentStore — где сайт хранит состояние оплаты заявки.
type PaymentStore interface {
	Get(ctx context.Context, id int) (domain.Booking, error)
	StartPayment(ctx context.Context, id int, amount int) error
	ResolvePayment(ctx context.Context, id int, status domain.PaymentStatus) (bool, error)
	ListPendingPayments(ctx context.Context) ([]domain.Booking, error)
}

// Онлайн оплачивается запись вокала. Всё, где есть сведение (пакет, сведение, трек под ключ),
// остаётся заявкой: цена зависит от сложности.
var payableServices = map[string]bool{"vocal": true}

var ErrPaymentsDisabled = errors.New("online payments are not configured")

// WithPayments включает онлайн-оплату. returnBase — публичный адрес сайта,
// куда ЮKassa вернёт клиента после оплаты.
func (s *Service) WithPayments(gateway PaymentGateway, store PaymentStore, returnBase string) *Service {
	s.payments = gateway
	s.paymentStore = store
	s.returnBase = strings.TrimRight(strings.TrimSpace(returnBase), "/")
	return s
}

func (s *Service) paymentsEnabled() bool {
	return s.payments != nil && s.paymentStore != nil && s.returnBase != ""
}

const startPaymentTimeout = 10 * time.Second

// startPayment просит бота создать платёж. false — оплату создать не вышло
// или она не нужна: тогда заявка уходит как раньше, без оплаты.
func (s *Service) startPayment(ctx context.Context, booking domain.Booking, items []ServiceItem) (botsync.PaymentResult, bool) {
	if !s.paymentsEnabled() || booking.DesiredDate.Year() <= 1970 {
		return botsync.PaymentResult{}, false
	}
	payable := false
	gwItems := make([]botsync.PaymentItem, 0, len(items))
	for _, it := range items {
		payable = payable || payableServices[it.ID]
		gwItems = append(gwItems, botsync.PaymentItem{ID: it.ID, Hours: it.Hours, Engineer: it.Engineer})
	}
	if !payable {
		return botsync.PaymentResult{}, false
	}

	payCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), startPaymentTimeout)
	defer cancel()
	returnURL := fmt.Sprintf("%s/?payment=%d", s.returnBase, booking.ID)
	res, err := s.payments.StartPayment(payCtx, booking, gwItems, returnURL)
	if err != nil || res.Status != "pending" || res.PayURL == "" {
		slog.Warn("online payment not created, booking goes as a request", "booking_id", booking.ID, "status", res.Status, "error", err)
		return botsync.PaymentResult{}, false
	}
	if err := s.paymentStore.StartPayment(payCtx, booking.ID, res.Amount); err != nil {
		slog.Error("payment created but not saved", "booking_id", booking.ID, "error", err)
		return botsync.PaymentResult{}, false
	}
	return res, true
}

// CheckPayment — состояние оплаты заявки; ожидающую оплату сверяет с ботом.
func (s *Service) CheckPayment(ctx context.Context, id int) (domain.Booking, error) {
	if !s.paymentsEnabled() {
		return domain.Booking{}, ErrPaymentsDisabled
	}
	b, err := s.paymentStore.Get(ctx, id)
	if err != nil {
		return domain.Booking{}, err
	}
	if b.PaymentStatus != domain.PaymentPending {
		return b, nil
	}
	if s.refreshPayment(ctx, b) {
		return s.paymentStore.Get(ctx, id)
	}
	return b, nil
}

// refreshPayment спрашивает бота и применяет ответ. true — статус изменился.
func (s *Service) refreshPayment(ctx context.Context, b domain.Booking) bool {
	ctx, cancel := context.WithTimeout(ctx, externalTimeout)
	defer cancel()
	status, err := s.payments.PaymentStatus(ctx, b.ID)
	if err != nil {
		slog.Warn("payment status check failed", "booking_id", b.ID, "error", err)
		return false
	}

	var next domain.PaymentStatus
	switch status {
	case "paid":
		next = domain.PaymentPaid
	case "expired", "paid_late":
		// paid_late: заплатили, когда время уже отпустили — бот сам написал админу
		next = domain.PaymentExpired
	case "none":
		// В боте нет такого платежа (например, база бота пересоздана)
		if time.Since(b.CreatedAt) < 5*time.Minute {
			return false
		}
		next = domain.PaymentExpired
	default:
		return false
	}

	changed, err := s.paymentStore.ResolvePayment(context.WithoutCancel(ctx), b.ID, next)
	if err != nil {
		slog.Error("resolve payment failed", "booking_id", b.ID, "error", err)
		return false
	}
	if changed && next == domain.PaymentPaid && s.notifier != nil {
		b.PaymentStatus = domain.PaymentPaid
		go s.notifier.NotifyNewBooking(context.WithoutCancel(ctx), b)
	}
	if changed {
		slog.Info("booking payment resolved", "booking_id", b.ID, "payment_status", next)
	}
	return changed
}

// WatchPayments раз в минуту сверяет ожидающие оплаты с ботом:
// оплаченные — уведомление админам, просроченные — заявка отменяется, время свободно.
func (s *Service) WatchPayments(ctx context.Context) {
	if !s.paymentsEnabled() {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pending, err := s.paymentStore.ListPendingPayments(ctx)
			if err != nil {
				slog.Warn("list pending payments failed", "error", err)
				continue
			}
			for _, b := range pending {
				s.refreshPayment(ctx, b)
			}
		}
	}
}
