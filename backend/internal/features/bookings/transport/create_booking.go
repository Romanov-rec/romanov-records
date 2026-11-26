package bookings_transport

import (
	"errors"
	"net/http"
	transport_http "romanov/backend/internal/core/transport/http"
	bookings_repository "romanov/backend/internal/features/bookings/repository"
	bookings_service "romanov/backend/internal/features/bookings/service"
)

func (h *BookingHTTPHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req CreateBookingRequest
	if err := transport_http.DecodeJSON(w, r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	durationHours := req.DurationHours
	if durationHours < 1 {
		durationHours = 1
	}

	input := bookings_service.CreateBookingInput{
		FullName:         req.FullName,
		PhoneNumber:      req.PhoneNumber,
		TelegramUsername: req.TelegramUsername,
		DesiredDate:      req.DesiredDate,
		DesiredTime:      req.DesiredTime,
		DurationHours:    durationHours,
		RequestDetails:   req.RequestDetails,
		Comment:          req.Comment,
	}
	for _, item := range req.Services {
		input.Items = append(input.Items, bookings_service.ServiceItem{ID: item.ID, Hours: item.Hours, Engineer: item.Engineer})
	}

	result, err := h.service.CreateBookingWithPayment(r.Context(), input)
	if err != nil {
		if errors.Is(err, bookings_repository.ErrBookingConflict) {
			http.Error(w, "Выбранное время уже занято. Пожалуйста, выберите другой слот.", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create booking", http.StatusBadRequest)
		return
	}

	transport_http.WriteJSON(w, http.StatusCreated, CreateBookingResponse{
		ID:     result.ID,
		PayURL: result.PayURL,
		Amount: result.Amount,
	})
}
