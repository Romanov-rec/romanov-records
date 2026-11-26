package bookings_repository

import (
	"context"
	"errors"
	"fmt"
	"romanov/backend/internal/core/domain"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrBookingNotFound   = errors.New("booking not found")
	ErrBookingConflict   = errors.New("booking time conflicts with an existing booking")
	ErrInvalidTransition = errors.New("invalid booking status transition")
)

type Repository interface {
	Create(ctx context.Context, booking domain.Booking) (int, error)
	List(ctx context.Context) ([]domain.Booking, error)
	GetBusyTimes(ctx context.Context, date time.Time) ([]string, error)
	UpdateStatus(ctx context.Context, id int, status string) error
}

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type PostgresRepository struct {
	db DB
}

func NewPostgresRepository(db DB) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	booking domain.Booking,
) (int, error) {
	sqlQuery := `
	INSERT INTO romanov.bookings(
		full_name,
		phone_number,
		telegram_username,
		desired_date,
		desired_time,
		duration_hours,
		request_details,
		comment,
		status,
		created_at,
		updated_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	RETURNING id
	`
	var id int

	err := r.db.QueryRow(
		ctx,
		sqlQuery,
		booking.FullName,
		booking.PhoneNumber,
		booking.TelegramUsername,
		booking.DesiredDate,
		booking.DesiredTime,
		booking.DurationHours,
		booking.RequestDetails,
		booking.Comment,
		booking.Status,
		booking.CreatedAt,
		booking.UpdatedAt,
	).Scan(&id)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" && pgErr.ConstraintName == "bookings_no_overlapping_times" {
			return 0, ErrBookingConflict
		}
		return 0, fmt.Errorf("create booking: %w", err)
	}

	return id, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]domain.Booking, error) {
	sqlQuery := `
		SELECT
			id,
			full_name,
			phone_number,
			telegram_username,
			desired_date,
			desired_time,
			duration_hours,
			request_details,
			comment,
			status,
			payment_status,
			payment_amount,
			created_at,
			updated_at
		FROM romanov.bookings
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]domain.Booking, 0)

	for rows.Next() {
		var booking domain.Booking

		if err := rows.Scan(
			&booking.ID,
			&booking.FullName,
			&booking.PhoneNumber,
			&booking.TelegramUsername,
			&booking.DesiredDate,
			&booking.DesiredTime,
			&booking.DurationHours,
			&booking.RequestDetails,
			&booking.Comment,
			&booking.Status,
			&booking.PaymentStatus,
			&booking.PaymentAmount,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}

		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}

	return bookings, nil
}

func (r *PostgresRepository) GetBusyTimes(ctx context.Context, date time.Time) ([]string, error) {
	sqlQuery := `
		SELECT
			GREATEST(desired_date + desired_time, $1::date::timestamp),
			LEAST(
				desired_date + desired_time + duration_hours * INTERVAL '1 hour',
				($1::date + 1)::timestamp
			)
		FROM romanov.bookings
		WHERE status != 'cancelled'
		  AND desired_date > DATE '1970-01-01'
		  AND tsrange(
			desired_date + desired_time,
			desired_date + desired_time + duration_hours * INTERVAL '1 hour',
			'[)'
		  ) && tsrange($1::date::timestamp, ($1::date + 1)::timestamp, '[)')
		ORDER BY desired_time
	`

	rows, err := r.db.Query(ctx, sqlQuery, date)
	if err != nil {
		return nil, fmt.Errorf("get busy times: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	slots := make([]string, 0)

	for rows.Next() {
		var start, end time.Time
		if err := rows.Scan(&start, &end); err != nil {
			return nil, fmt.Errorf("scan busy time: %w", err)
		}
		for slotTime := start; slotTime.Before(end); slotTime = slotTime.Add(30 * time.Minute) {
			slot := fmt.Sprintf("%02d:%02d", slotTime.Hour(), slotTime.Minute())
			if _, exists := seen[slot]; !exists {
				seen[slot] = struct{}{}
				slots = append(slots, slot)
			}
		}
	}

	return slots, rows.Err()
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id int, status string) error {
	sqlQuery := `
		WITH current AS MATERIALIZED (
			SELECT id, status
			FROM romanov.bookings
			WHERE id = $1
			FOR UPDATE
		), updated AS (
			UPDATE romanov.bookings AS booking
			SET status = $2, updated_at = NOW()
			FROM current
			WHERE booking.id = current.id
			  AND (
				(current.status = 'new' AND $2 IN ('confirmed', 'completed', 'cancelled'))
				OR (current.status = 'confirmed' AND $2 IN ('new', 'completed', 'cancelled'))
				OR (current.status IN ('completed', 'cancelled') AND $2 = 'new')
			  )
			RETURNING booking.id
		)
		SELECT
			EXISTS(SELECT 1 FROM current),
			EXISTS(SELECT 1 FROM updated)
	`

	var exists, updated bool
	err := r.db.QueryRow(ctx, sqlQuery, id, status).Scan(&exists, &updated)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" && pgErr.ConstraintName == "bookings_no_overlapping_times" {
			return ErrBookingConflict
		}
		return fmt.Errorf("update booking status: %w", err)
	}

	if !exists {
		return ErrBookingNotFound
	}
	if !updated {
		return ErrInvalidTransition
	}

	return nil
}

// ---------- Онлайн-оплата ----------

const bookingColumns = `
	id, full_name, phone_number, telegram_username, desired_date, desired_time,
	duration_hours, request_details, comment, status, payment_status, payment_amount,
	created_at, updated_at`

func scanBooking(row pgx.Row) (domain.Booking, error) {
	var b domain.Booking
	err := row.Scan(
		&b.ID, &b.FullName, &b.PhoneNumber, &b.TelegramUsername, &b.DesiredDate, &b.DesiredTime,
		&b.DurationHours, &b.RequestDetails, &b.Comment, &b.Status, &b.PaymentStatus, &b.PaymentAmount,
		&b.CreatedAt, &b.UpdatedAt,
	)
	return b, err
}

// Get возвращает одну заявку.
func (r *PostgresRepository) Get(ctx context.Context, id int) (domain.Booking, error) {
	b, err := scanBooking(r.db.QueryRow(ctx, `SELECT `+bookingColumns+` FROM romanov.bookings WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return domain.Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return b, nil
}

// StartPayment помечает заявку «ждёт оплату» с суммой, посчитанной ботом.
func (r *PostgresRepository) StartPayment(ctx context.Context, id int, amount int) error {
	var updated bool
	err := r.db.QueryRow(ctx, `
		WITH u AS (
			UPDATE romanov.bookings
			SET payment_status = 'pending', payment_amount = $2, updated_at = NOW()
			WHERE id = $1
			RETURNING id
		)
		SELECT EXISTS(SELECT 1 FROM u)`, id, amount).Scan(&updated)
	if err != nil {
		return fmt.Errorf("start payment: %w", err)
	}
	if !updated {
		return ErrBookingNotFound
	}
	return nil
}

// ResolvePayment переводит ожидающую оплату в paid или expired.
// Возвращает true, только если статус изменился именно этим вызовом —
// так уведомление об оплате уходит ровно один раз, даже при параллельных проверках.
// expired заодно отменяет заявку, чтобы время освободилось и на сайте.
func (r *PostgresRepository) ResolvePayment(ctx context.Context, id int, status domain.PaymentStatus) (bool, error) {
	if status != domain.PaymentPaid && status != domain.PaymentExpired {
		return false, fmt.Errorf("resolve payment: unexpected status %q", status)
	}
	var changed bool
	err := r.db.QueryRow(ctx, `
		WITH u AS (
			UPDATE romanov.bookings
			SET payment_status = $2,
			    status = CASE WHEN $2 = 'expired' THEN 'cancelled' ELSE status END,
			    updated_at = NOW()
			WHERE id = $1 AND payment_status = 'pending'
			RETURNING id
		)
		SELECT EXISTS(SELECT 1 FROM u)`, id, string(status)).Scan(&changed)
	if err != nil {
		return false, fmt.Errorf("resolve payment: %w", err)
	}
	return changed, nil
}

// ListPendingPayments — заявки, по которым ждём оплату.
func (r *PostgresRepository) ListPendingPayments(ctx context.Context) ([]domain.Booking, error) {
	rows, err := r.db.Query(ctx, `SELECT `+bookingColumns+` FROM romanov.bookings WHERE payment_status = 'pending' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list pending payments: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Booking, 0)
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
