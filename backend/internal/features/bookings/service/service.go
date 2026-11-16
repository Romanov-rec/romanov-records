package bookings_service

import (
	"context"
	"fmt"
	"log/slog"
	"romanov/backend/internal/core/domain"
	bookings_repository "romanov/backend/internal/features/bookings/repository"
	"sort"
	"time"

	"github.com/go-playground/validator/v10"
)

// Notifier is satisfied by anything that can be told a new booking happened.
// Kept tiny so the service stays decoupled from the transport (Telegram, email, ...).
type Notifier interface {
	NotifyNewBooking(ctx context.Context, booking domain.Booking)
}

// ExternalSchedule — расписание студии вне сайта (Telegram-бот).
// Сайт не даёт записаться на время, занятое там, и сам отправляет туда свои заявки.
type ExternalSchedule interface {
	BusyTimes(ctx context.Context, date string) ([]string, error)
	PushBooking(ctx context.Context, booking domain.Booking) error
}

type Service struct {
	repo     bookings_repository.Repository
	validate *validator.Validate
	notifier Notifier
	external ExternalSchedule
}

func NewService(repo bookings_repository.Repository, notifier Notifier) *Service {
	v := validator.New()

	return &Service{
		repo:     repo,
		validate: v,
		notifier: notifier,
	}
}

// WithExternalSchedule подключает расписание бота. Без него сайт работает автономно.
func (s *Service) WithExternalSchedule(external ExternalSchedule) *Service {
	s.external = external
	return s
}

const externalTimeout = 4 * time.Second

// externalConflict проверяет, не занято ли время в боте. Если бот недоступен,
// запись не блокируем: лучше принять заявку и разобраться вручную, чем потерять клиента.
func (s *Service) externalConflict(ctx context.Context, date, clock time.Time, hours int) bool {
	if s.external == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, externalTimeout)
	defer cancel()

	start := time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), 0, 0, time.UTC)
	end := start.Add(time.Duration(hours) * time.Hour)
	busyByDay := map[string]map[string]struct{}{}
	for slot := start; slot.Before(end); slot = slot.Add(30 * time.Minute) {
		day := slot.Format("2006-01-02")
		busy, ok := busyByDay[day]
		if !ok {
			list, err := s.external.BusyTimes(ctx, day)
			if err != nil {
				slog.Warn("external schedule unavailable, skipping conflict check", "date", day, "error", err)
				return false
			}
			busy = make(map[string]struct{}, len(list))
			for _, t := range list {
				busy[t] = struct{}{}
			}
			busyByDay[day] = busy
		}
		if _, taken := busy[slot.Format("15:04")]; taken {
			return true
		}
	}
	return false
}

func (s *Service) pushExternal(ctx context.Context, booking domain.Booking) {
	if s.external == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.external.PushBooking(ctx, booking); err != nil {
		slog.Warn("push booking to external schedule failed", "booking_id", booking.ID, "error", err)
	}
}

func (s *Service) CreateBooking(
	ctx context.Context,
	input CreateBookingInput,
) (int, error) {
	if err := s.validate.Struct(input); err != nil {
		return 0, fmt.Errorf("validate CreateBooking: %w", err)
	}

	desiredDate, err := time.Parse("2006-01-02", input.DesiredDate)
	if err != nil {
		return 0, fmt.Errorf("desiredDate parse: %w", err)
	}

	desiredTime, err := time.Parse("15:04", input.DesiredTime)
	if err != nil {
		return 0, fmt.Errorf("desiredTime parse: %w", err)
	}
	if desiredTime.Minute() != 0 && desiredTime.Minute() != 30 {
		return 0, fmt.Errorf("desiredTime must use a 30-minute boundary")
	}

	if s.externalConflict(ctx, desiredDate, desiredTime, input.DurationHours) {
		return 0, fmt.Errorf("%w: время занято в расписании студии", bookings_repository.ErrBookingConflict)
	}

	now := time.Now()
	booking := domain.Booking{
		FullName:         input.FullName,
		PhoneNumber:      input.PhoneNumber,
		TelegramUsername: input.TelegramUsername,
		DesiredDate:      desiredDate,
		DesiredTime:      desiredTime,
		DurationHours:    input.DurationHours,
		RequestDetails:   input.RequestDetails,
		Comment:          input.Comment,
		Status:           domain.BookingStatusNew,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	id, err := s.repo.Create(ctx, booking)
	if err != nil {
		return 0, err
	}

	booking.ID = id
	if s.external != nil {
		go s.pushExternal(context.WithoutCancel(ctx), booking)
	}

	if s.notifier != nil {
		// Fire-and-forget: don't block the HTTP response on Telegram I/O,
		// and don't fail the booking just because the notifier is down.
		go s.notifier.NotifyNewBooking(context.WithoutCancel(ctx), booking)
	}

	return id, nil
}

func (s *Service) ListBookings(ctx context.Context) ([]domain.Booking, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetBusyTimes(ctx context.Context, date string) ([]string, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	}

	local, err := s.repo.GetBusyTimes(ctx, d)
	if err != nil || s.external == nil {
		return local, err
	}

	extCtx, cancel := context.WithTimeout(ctx, externalTimeout)
	defer cancel()
	remote, extErr := s.external.BusyTimes(extCtx, d.Format("2006-01-02"))
	if extErr != nil {
		slog.Warn("external schedule unavailable, showing site bookings only", "date", date, "error", extErr)
		return local, nil
	}

	seen := make(map[string]struct{}, len(local)+len(remote))
	merged := make([]string, 0, len(local)+len(remote))
	for _, t := range append(local, remote...) {
		if _, dup := seen[t]; !dup {
			seen[t] = struct{}{}
			merged = append(merged, t)
		}
	}
	sort.Strings(merged)
	return merged, nil
}

func (s *Service) UpdateBookingStatus(ctx context.Context, id int, status string) error {
	bookingStatus := domain.BookingStatus(status)
	if !bookingStatus.IsValid() {
		return fmt.Errorf("недопустимый статус: %s", status)
	}

	if err := s.repo.UpdateStatus(ctx, id, string(bookingStatus)); err != nil {
		return err
	}

	if s.external != nil {
		go s.syncStatus(context.WithoutCancel(ctx), id)
	}
	return nil
}

// syncStatus переотправляет заявку целиком: отмена убирает её из расписания бота,
// возврат в работу — ставит обратно.
func (s *Service) syncStatus(ctx context.Context, id int) {
	all, err := s.repo.List(ctx)
	if err != nil {
		slog.Warn("sync status: list bookings failed", "booking_id", id, "error", err)
		return
	}
	for _, b := range all {
		if b.ID == id {
			s.pushExternal(ctx, b)
			return
		}
	}
}
