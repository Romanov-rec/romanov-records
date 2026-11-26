package domain

import "time"

type BookingStatus string

const (
	BookingStatusNew       BookingStatus = "new"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCompleted BookingStatus = "completed"
	BookingStatusCancelled BookingStatus = "cancelled"
)

func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusNew, BookingStatusConfirmed, BookingStatusCompleted, BookingStatusCancelled:
		return true
	default:
		return false
	}
}

func (s BookingStatus) CanTransitionTo(next BookingStatus) bool {
	switch s {
	case BookingStatusNew:
		return next == BookingStatusConfirmed || next == BookingStatusCompleted || next == BookingStatusCancelled
	case BookingStatusConfirmed:
		return next == BookingStatusNew || next == BookingStatusCompleted || next == BookingStatusCancelled
	case BookingStatusCompleted, BookingStatusCancelled:
		return next == BookingStatusNew
	default:
		return false
	}
}

// PaymentStatus — онлайн-оплата записи с сайта.
type PaymentStatus string

const (
	PaymentNone    PaymentStatus = "none"    // оплата не требовалась — обычная заявка
	PaymentPending PaymentStatus = "pending" // клиента отправили платить, время держим
	PaymentPaid    PaymentStatus = "paid"
	PaymentExpired PaymentStatus = "expired" // не оплатили вовремя, время освобождено
)

type Booking struct {
	ID               int
	FullName         string
	PhoneNumber      string
	TelegramUsername string
	DesiredDate      time.Time
	DesiredTime      time.Time
	DurationHours    int
	RequestDetails   string
	Comment          string
	Status           BookingStatus
	PaymentStatus    PaymentStatus
	PaymentAmount    int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
