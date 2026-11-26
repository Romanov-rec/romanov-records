package bookings_transport

import (
	"errors"
	"net/http"
	transport_http "romanov/backend/internal/core/transport/http"
	bookings_repository "romanov/backend/internal/features/bookings/repository"
	bookings_service "romanov/backend/internal/features/bookings/service"
	"strconv"
)

// GetPaymentStatus — клиент вернулся с ЮKassa: показываем, прошла ли оплата.
// Отдаём только статус и сумму, без личных данных.
func (h *BookingHTTPHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	booking, err := h.service.CheckPayment(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, bookings_repository.ErrBookingNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, bookings_service.ErrPaymentsDisabled):
			http.Error(w, "payments disabled", http.StatusNotFound)
		default:
			http.Error(w, "failed to check payment", http.StatusInternalServerError)
		}
		return
	}

	transport_http.WriteJSON(w, http.StatusOK, PaymentStatusResponse{
		Status: booking.PaymentStatus,
		Amount: booking.PaymentAmount,
	})
}
