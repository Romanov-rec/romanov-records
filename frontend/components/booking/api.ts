const API = "/api/v1";

export interface CreateBookingRequest {
  full_name: string;
  phone_number: string;
  telegram_username: string;
  desired_date: string;
  desired_time: string;
  duration_hours: number;
  request_details: string;
  comment: string;
  services: { id: string; hours?: number; engineer?: boolean }[];
}

export interface CreateBookingResult {
  id: number;
  pay_url?: string;
  amount?: number;
}

export type PaymentStatus = "none" | "pending" | "paid" | "expired";

export async function fetchBusyTimes(date: string): Promise<string[]> {
  const response = await fetch(`${API}/bookings/busy?date=${encodeURIComponent(date)}`);
  if (!response.ok) {
    throw new Error("availability request failed");
  }

  const data: unknown = await response.json();
  if (!isBusyTimesResponse(data)) {
    throw new Error("invalid availability response");
  }
  return data.busy;
}

export async function createBooking(request: CreateBookingRequest): Promise<CreateBookingResult> {
  const response = await fetch(`${API}/bookings`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    const message = (await response.text()).trim();
    throw new Error(message || "Не удалось отправить заявку. Попробуйте ещё раз.");
  }
  return (await response.json()) as CreateBookingResult;
}

export async function fetchPaymentStatus(id: number): Promise<{ status: PaymentStatus; amount: number }> {
  const response = await fetch(`${API}/bookings/${id}/payment`, { cache: "no-store" });
  if (!response.ok) throw new Error("payment status request failed");
  return response.json();
}

function isBusyTimesResponse(value: unknown): value is { busy: string[] } {
  if (typeof value !== "object" || value === null || !("busy" in value)) return false;
  return Array.isArray(value.busy) && value.busy.every((slot) => typeof slot === "string");
}
