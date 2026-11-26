export const TEAL = "#1db8a6";

// payable — запись вокала оплачивается онлайн сразу; всё, где есть сведение, — заявка: цена зависит от сложности
export const SERVICES = [
  { id: "vocal", label: "Запись вокала", hasHours: true, payable: true },
  { id: "package", label: "Пакет запись + сведение", hasHours: true, payable: false },
  { id: "mixing", label: "Сведение + мастеринг", hasHours: false, payable: false },
  { id: "turnkey", label: "Трек под ключ", hasHours: false, payable: false },
] as const;

export type ServiceID = (typeof SERVICES)[number]["id"];
// engineer — только для вокала: со звукорежиссёром или аренда без него
export type ServiceSelection = Record<ServiceID, { checked: boolean; hours: string; engineer: boolean }>;

export const createEmptyServices = (): ServiceSelection =>
  Object.fromEntries(
    SERVICES.map((service) => [service.id, { checked: false, hours: "1", engineer: true }]),
  ) as ServiceSelection;

// Цены для подсказки в форме. Итоговую сумму считает сервер по тарифам студии.
export const VOCAL_RATE = { engineer: 2500, rent: 1750 } as const;

export const hoursOf = (value: string) => Math.min(24, Math.max(1, Number.parseInt(value, 10) || 1));

export function estimateTotal(services: ServiceSelection) {
  let total = 0;
  if (services.vocal.checked) {
    total += (services.vocal.engineer ? VOCAL_RATE.engineer : VOCAL_RATE.rent) * hoursOf(services.vocal.hours);
  }
  return total;
}

export const formatRub = (value: number) => `${value.toLocaleString("ru-RU")} ₽`;

export const TIME_SLOTS = Array.from({ length: 48 }, (_, index) => {
  const hour = Math.floor(index / 2);
  const minute = index % 2 === 0 ? "00" : "30";
  return `${String(hour).padStart(2, "0")}:${minute}`;
});

export function isNightSlot(slot: string) {
  const hour = Number.parseInt(slot.split(":")[0], 10);
  return hour >= 22 || hour <= 9;
}

export function sameDay(a: Date, b: Date) {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function isSlotDisabled(slot: string, selectedDate: Date | null, busyTimes: string[]) {
  if (busyTimes.includes(slot)) return true;
  if (!selectedDate) return false;

  const now = new Date();
  if (!sameDay(selectedDate, now)) return false;

  const [hour, minute] = slot.split(":").map(Number);
  const minimumStart = now.getTime() + 60 * 60 * 1000;
  const slotStart = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
    hour,
    minute,
  ).getTime();
  return slotStart < minimumStart;
}

export function localDateString(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

// Номер по мере ввода: 8… → +7…, 9… → +79…, без +7 в начале → +7…; пробелы и скобки убираем.
// Иностранный номер с «+» в начале остаётся как есть.
export function typePhone(value: string) {
  const s = value.trim();
  let d = s.replace(/\D/g, "");
  if (!d) return s.startsWith("+") ? "+" : "";
  if (d[0] === "8") d = "7" + d.slice(1);
  else if (d[0] === "9") d = "7" + d;
  else if (d[0] !== "7" && !s.startsWith("+")) d = "7" + d;
  return "+" + d.slice(0, d[0] === "7" ? 11 : 15);
}
