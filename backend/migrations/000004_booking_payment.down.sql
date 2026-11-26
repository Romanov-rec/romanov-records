DROP INDEX IF EXISTS romanov.bookings_payment_pending_idx;

ALTER TABLE romanov.bookings
    DROP CONSTRAINT IF EXISTS bookings_payment_status_check,
    DROP COLUMN IF EXISTS payment_amount,
    DROP COLUMN IF EXISTS payment_status;
