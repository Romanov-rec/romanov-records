package bookings_service

type CreateBookingInput struct {
	FullName         string `validate:"required,min=3,max=200"`
	PhoneNumber      string `validate:"required,min=10,max=15,startswith=+"`
	TelegramUsername string `validate:"omitempty,min=4,max=100,startswith=@"`
	DesiredDate      string `validate:"required"`
	DesiredTime      string `validate:"required"`
	DurationHours    int    `validate:"required,min=1,max=24"`
	RequestDetails   string `validate:"required"`
	Comment          string `validate:"omitempty"`
	// Что выбрал клиент — по этому бот считает сумму онлайн-оплаты
	Items []ServiceItem `validate:"omitempty,max=10,dive"`
}

type ServiceItem struct {
	ID       string `validate:"required,oneof=vocal package mixing turnkey"`
	Hours    int    `validate:"omitempty,min=1,max=24"`
	Engineer *bool
}
