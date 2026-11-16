package bookings_service

import (
	"context"
	"errors"
	"reflect"
	"romanov/backend/internal/core/domain"
	bookings_repository "romanov/backend/internal/features/bookings/repository"
	"sync"
	"testing"
	"time"
)

type fakeExternal struct {
	mu     sync.Mutex
	busy   map[string][]string
	err    error
	pushed chan domain.Booking
	asked  []string
}

func (f *fakeExternal) BusyTimes(_ context.Context, date string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, date)
	return f.busy[date], f.err
}

func (f *fakeExternal) PushBooking(_ context.Context, b domain.Booking) error {
	f.pushed <- b
	return nil
}

type busyRepository struct {
	fakeRepository
	busy []string
	list []domain.Booking
}

func (r *busyRepository) GetBusyTimes(context.Context, time.Time) ([]string, error) {
	return r.busy, nil
}

func (r *busyRepository) List(context.Context) ([]domain.Booking, error) { return r.list, nil }

func TestCreateBookingRejectsTimeBusyInBot(t *testing.T) {
	t.Parallel()

	ext := &fakeExternal{busy: map[string][]string{"2026-10-10": {"16:30"}}, pushed: make(chan domain.Booking, 1)}
	repo := &fakeRepository{createID: 5}
	service := NewService(repo, nil).WithExternalSchedule(ext)

	_, err := service.CreateBooking(context.Background(), validCreateInput()) // 15:30–17:30
	if !errors.Is(err, bookings_repository.ErrBookingConflict) {
		t.Fatalf("CreateBooking() error = %v, want ErrBookingConflict", err)
	}
	if repo.created.FullName != "" {
		t.Fatal("booking was saved despite the conflict")
	}
}

func TestCreateBookingChecksNextDayAfterMidnight(t *testing.T) {
	t.Parallel()

	ext := &fakeExternal{busy: map[string][]string{"2026-10-11": {"00:30"}}, pushed: make(chan domain.Booking, 1)}
	input := validCreateInput()
	input.DesiredTime = "23:00"
	_, err := NewService(&fakeRepository{}, nil).WithExternalSchedule(ext).CreateBooking(context.Background(), input)
	if !errors.Is(err, bookings_repository.ErrBookingConflict) {
		t.Fatalf("CreateBooking() error = %v, want conflict on the next day", err)
	}
}

func TestCreateBookingPushesToBotAndToleratesBotOutage(t *testing.T) {
	t.Parallel()

	ext := &fakeExternal{err: errors.New("bot is down"), pushed: make(chan domain.Booking, 1)}
	service := NewService(&fakeRepository{createID: 9}, nil).WithExternalSchedule(ext)

	id, err := service.CreateBooking(context.Background(), validCreateInput())
	if err != nil || id != 9 {
		t.Fatalf("CreateBooking() = (%d, %v), want (9, nil) when the bot is unreachable", id, err)
	}
	select {
	case b := <-ext.pushed:
		if b.ID != 9 || b.Status != domain.BookingStatusNew {
			t.Fatalf("pushed booking = %#v", b)
		}
	case <-time.After(time.Second):
		t.Fatal("booking was not pushed to the bot")
	}
}

func TestGetBusyTimesMergesBotSchedule(t *testing.T) {
	t.Parallel()

	ext := &fakeExternal{busy: map[string][]string{"2026-10-10": {"12:00", "10:30"}}}
	repo := &busyRepository{busy: []string{"10:00", "10:30"}}
	got, err := NewService(repo, nil).WithExternalSchedule(ext).GetBusyTimes(context.Background(), "2026-10-10")
	if err != nil {
		t.Fatalf("GetBusyTimes() error = %v", err)
	}
	if want := []string{"10:00", "10:30", "12:00"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GetBusyTimes() = %v, want %v", got, want)
	}

	ext.err = errors.New("down")
	got, err = NewService(repo, nil).WithExternalSchedule(ext).GetBusyTimes(context.Background(), "2026-10-10")
	if err != nil || !reflect.DeepEqual(got, []string{"10:00", "10:30"}) {
		t.Fatalf("GetBusyTimes() with bot down = (%v, %v), want site slots only", got, err)
	}
}

func TestUpdateBookingStatusResyncsBooking(t *testing.T) {
	t.Parallel()

	ext := &fakeExternal{pushed: make(chan domain.Booking, 1)}
	repo := &busyRepository{list: []domain.Booking{{ID: 3, Status: domain.BookingStatusCancelled}}}
	if err := NewService(repo, nil).WithExternalSchedule(ext).UpdateBookingStatus(context.Background(), 3, "cancelled"); err != nil {
		t.Fatalf("UpdateBookingStatus() error = %v", err)
	}
	select {
	case b := <-ext.pushed:
		if b.ID != 3 || b.Status != domain.BookingStatusCancelled {
			t.Fatalf("pushed booking = %#v", b)
		}
	case <-time.After(time.Second):
		t.Fatal("status change was not pushed to the bot")
	}
}
