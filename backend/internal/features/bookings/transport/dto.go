package bookings_transport

import (
	"romanov/backend/internal/core/domain"
	"time"
)

type CreateBookingRequest struct {
	FullName         string `json:"full_name"`
	PhoneNumber      string `json:"phone_number"`
	TelegramUsername string `json:"telegram_username"`
	DesiredDate      string `json:"desired_date"`
	DesiredTime      string `json:"desired_time"`
	DurationHours    int    `json:"duration_hours"`
	RequestDetails   string `json:"request_details"`
	Comment          string `json:"comment"`
	// Выбранные услуги: по ним бот считает сумму онлайн-оплаты
	Services []ServiceItemRequest `json:"services"`
}

type ServiceItemRequest struct {
	ID       string `json:"id"`
	Hours    int    `json:"hours"`
	Engineer *bool  `json:"engineer"`
}

type CreateBookingResponse struct {
	ID int `json:"id"`
	// Есть, если запись оплачивается онлайн: фронтенд отправляет клиента на эту страницу ЮKassa
	PayURL string `json:"pay_url,omitempty"`
	Amount int    `json:"amount,omitempty"`
}

type PaymentStatusResponse struct {
	Status domain.PaymentStatus `json:"status"`
	Amount int                  `json:"amount"`
}

type BusyTimesResponse struct {
	Busy []string `json:"busy"`
}

type UpdateBookingStatusRequest struct {
	Status string `json:"status"`
}

type BookingResponse struct {
	ID               int                  `json:"id"`
	FullName         string               `json:"full_name"`
	PhoneNumber      string               `json:"phone_number"`
	TelegramUsername string               `json:"telegram_username,omitempty"`
	DesiredDate      string               `json:"desired_date"`
	DesiredTime      string               `json:"desired_time"`
	DurationHours    int                  `json:"duration_hours"`
	RequestDetails   string               `json:"request_details"`
	Comment          string               `json:"comment,omitempty"`
	Status           domain.BookingStatus `json:"status"`
	PaymentStatus    domain.PaymentStatus `json:"payment_status"`
	PaymentAmount    int                  `json:"payment_amount"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

func NewBookingResponse(booking domain.Booking) BookingResponse {
	return BookingResponse{
		ID:               booking.ID,
		FullName:         booking.FullName,
		PhoneNumber:      booking.PhoneNumber,
		TelegramUsername: booking.TelegramUsername,
		DesiredDate:      booking.DesiredDate.Format("2006-01-02"),
		DesiredTime:      booking.DesiredTime.Format("15:04"),
		DurationHours:    booking.DurationHours,
		RequestDetails:   booking.RequestDetails,
		Comment:          booking.Comment,
		Status:           booking.Status,
		PaymentStatus:    booking.PaymentStatus,
		PaymentAmount:    booking.PaymentAmount,
		CreatedAt:        booking.CreatedAt,
		UpdatedAt:        booking.UpdatedAt,
	}
}
