-- Онлайн-оплата записи с сайта (через ЮKassa Telegram-бота студии).
-- none — оплата не требовалась (заявка), pending — ждём оплату, paid — оплачено,
-- expired — не оплатили вовремя, время освобождено.
ALTER TABLE romanov.bookings
    ADD COLUMN payment_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN payment_amount INTEGER NOT NULL DEFAULT 0;

ALTER TABLE romanov.bookings
    ADD CONSTRAINT bookings_payment_status_check
    CHECK (payment_status IN ('none', 'pending', 'paid', 'expired'));

CREATE INDEX bookings_payment_pending_idx
    ON romanov.bookings (id)
    WHERE payment_status = 'pending';
