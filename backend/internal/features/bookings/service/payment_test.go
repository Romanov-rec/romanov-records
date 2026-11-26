package bookings_service

import (
	"context"
	"errors"
	"romanov/backend/internal/core/domain"
	"romanov/backend/internal/features/integrations/botsync"
	"sync"
	"testing"
	"time"
)

type fakeGateway struct {
	mu        sync.Mutex
	result    botsync.PaymentResult
	err       error
	status    string
	items     []botsync.PaymentItem
	returnURL string
	calls     int
}

func (g *fakeGateway) StartPayment(_ context.Context, _ domain.Booking, items []botsync.PaymentItem, returnURL string) (botsync.PaymentResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	g.items, g.returnURL = items, returnURL
	return g.result, g.err
}

func (g *fakeGateway) PaymentStatus(context.Context, int) (string, error) { return g.status, nil }

type fakeStore struct {
	mu       sync.Mutex
	booking  domain.Booking
	amount   int
	resolved []domain.PaymentStatus
}

func (s *fakeStore) Get(context.Context, int) (domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.booking, nil
}
func (s *fakeStore) StartPayment(_ context.Context, id int, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.booking.ID, s.booking.PaymentStatus, s.booking.PaymentAmount, s.amount = id, domain.PaymentPending, amount, amount
	return nil
}
func (s *fakeStore) ResolvePayment(_ context.Context, _ int, st domain.PaymentStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.booking.PaymentStatus != domain.PaymentPending {
		return false, nil
	}
	s.booking.PaymentStatus = st
	s.resolved = append(s.resolved, st)
	return true, nil
}
func (s *fakeStore) ListPendingPayments(context.Context) ([]domain.Booking, error) { return nil, nil }

func vocalInput() CreateBookingInput {
	in := validCreateInput()
	no := false
	in.Items = []ServiceItem{{ID: "vocal", Hours: 2, Engineer: &no}, {ID: "mixing"}}
	return in
}

func TestHourlyBookingReturnsPayURLAndDefersNotification(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{result: botsync.PaymentResult{Status: "pending", PayURL: "https://yoomoney.ru/pay/1", Amount: 3500}}
	store := &fakeStore{}
	notifications := make(chan domain.Booking, 1)
	svc := NewService(&fakeRepository{createID: 21}, channelNotifier{bookings: notifications}).
		WithPayments(gw, store, "https://romanov-records.ru/")

	res, err := svc.CreateBookingWithPayment(context.Background(), vocalInput())
	if err != nil {
		t.Fatalf("CreateBookingWithPayment() error = %v", err)
	}
	if res.ID != 21 || res.PayURL != "https://yoomoney.ru/pay/1" || res.Amount != 3500 {
		t.Fatalf("result = %#v", res)
	}
	if gw.returnURL != "https://romanov-records.ru/?payment=21" {
		t.Fatalf("returnURL = %q", gw.returnURL)
	}
	if len(gw.items) != 2 || gw.items[0].ID != "vocal" || gw.items[0].Engineer == nil || *gw.items[0].Engineer {
		t.Fatalf("items = %#v", gw.items)
	}
	if store.amount != 3500 {
		t.Fatalf("stored amount = %d", store.amount)
	}
	select {
	case <-notifications:
		t.Fatal("unpaid booking must not notify admins")
	case <-time.After(100 * time.Millisecond):
	}

	// Бот подтвердил оплату — уведомление уходит один раз
	gw.status = "paid"
	b, err := svc.CheckPayment(context.Background(), 21)
	if err != nil || b.PaymentStatus != domain.PaymentPaid {
		t.Fatalf("CheckPayment() = %#v, %v", b, err)
	}
	if _, err := svc.CheckPayment(context.Background(), 21); err != nil {
		t.Fatal(err)
	}
	select {
	case n := <-notifications:
		if n.PaymentStatus != domain.PaymentPaid {
			t.Fatalf("notification = %#v", n)
		}
	case <-time.After(time.Second):
		t.Fatal("paid booking was not notified")
	}
	select {
	case <-notifications:
		t.Fatal("paid booking notified twice")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestExpiredPaymentResolvesWithoutNotification(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{status: "expired"}
	store := &fakeStore{booking: domain.Booking{ID: 5, PaymentStatus: domain.PaymentPending, CreatedAt: time.Now()}}
	notifications := make(chan domain.Booking, 1)
	svc := NewService(&fakeRepository{}, channelNotifier{bookings: notifications}).WithPayments(gw, store, "https://x.ru")

	b, err := svc.CheckPayment(context.Background(), 5)
	if err != nil || b.PaymentStatus != domain.PaymentExpired {
		t.Fatalf("CheckPayment() = %#v, %v", b, err)
	}
	select {
	case <-notifications:
		t.Fatal("expired booking must not notify")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRequestOnlyServicesSkipPayment(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{}
	notifications := make(chan domain.Booking, 1)
	svc := NewService(&fakeRepository{createID: 3}, channelNotifier{bookings: notifications}).WithPayments(gw, &fakeStore{}, "https://x.ru")
	in := validCreateInput()
	in.Items = []ServiceItem{{ID: "turnkey"}}

	res, err := svc.CreateBookingWithPayment(context.Background(), in)
	if err != nil || res.PayURL != "" || gw.calls != 0 {
		t.Fatalf("result = %#v, err = %v, gateway calls = %d", res, err, gw.calls)
	}
	select {
	case <-notifications:
	case <-time.After(time.Second):
		t.Fatal("request must notify admins right away")
	}
}

func TestPaymentFailureFallsBackToRequest(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{err: errors.New("yookassa is down")}
	notifications := make(chan domain.Booking, 1)
	svc := NewService(&fakeRepository{createID: 8}, channelNotifier{bookings: notifications}).WithPayments(gw, &fakeStore{}, "https://x.ru")

	res, err := svc.CreateBookingWithPayment(context.Background(), vocalInput())
	if err != nil || res.ID != 8 || res.PayURL != "" {
		t.Fatalf("result = %#v, err = %v", res, err)
	}
	select {
	case <-notifications:
	case <-time.After(time.Second):
		t.Fatal("fallback request must notify admins")
	}
}
