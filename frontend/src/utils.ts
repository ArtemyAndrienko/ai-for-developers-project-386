// Чистые функции без обращения к API — покрываются vitest.

export const WINDOW_DAYS = 14;
export const LAST_EMAIL_KEY = "calendar.email";
export const OWNER_EMAIL_KEY = "calendar.ownerEmail";
export const OWNER_NAME_KEY = "calendar.ownerName";
export const HIDE_PAST_KEY = "calendar.hidePast";

export function isoDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function todayIso(now: Date = new Date()): string {
  return isoDate(now);
}

export function addDays(iso: string, n: number): string {
  const d = new Date(`${iso}T00:00:00`);
  d.setDate(d.getDate() + n);
  return isoDate(d);
}

// Понедельник недели, к которой относится дата (как в референсе: Пн–Вс).
export function weekStartMonday(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  const wd = (d.getDay() + 6) % 7; // 0 = понедельник
  return addDays(iso, -wd);
}

// Окно доступности: windowDays дней, начиная с сегодняшнего (Сценарий 1, шаг 2).
export function buildWindow(today: string, days = WINDOW_DAYS): string[] {
  return Array.from({ length: days }, (_, i) => addDays(today, i));
}

export function hmToMin(hm: string): number {
  const [h, m] = hm.split(":").map(Number);
  return h * 60 + m;
}

const slotLabels: Record<string, string> = {
  free: "Свободно",
  taken: "Занято",
  past: "Прошло",
};

export function slotLabel(status: string): string {
  return slotLabels[status] ?? status;
}

export const emailLike = (s: string) => /\S+@\S+\.\S+/.test(s.trim());

import type { BookingView, Slot } from "./types";

// Единый предикат «не предстоящая»: завершившаяся или отменённая Запись —
// группа «Прошедшие», без кнопки отмены (используется и для конфликтов).
export function isCancelledOrPast(b: Pick<BookingView, "past" | "state">): boolean {
  return b.past || b.state === "cancelled";
}

// Мягкий конфликт (GLOSSARY: «Конфликт»): пересечение интервала с будущей
// записью посетителя в ДРУГОЙ встрече. Бронированию не мешает — только
// предупреждаем. Отменённые Записи не пересекаются: их Интервалы свободны.
export function visitorClashes(
  bookings: BookingView[],
  meetingId: string,
  meetingDate: string,
  slotStart: string,
  slotEnd: string,
): BookingView[] {
  const s = hmToMin(slotStart);
  const e = hmToMin(slotEnd);
  return bookings.filter(
    (b) =>
      !isCancelledOrPast(b) &&
      b.meetingId !== meetingId &&
      b.date === meetingDate &&
      hmToMin(b.start) < e &&
      s < hmToMin(b.end),
  );
}

// Жёсткий конфликт (GLOSSARY: «Конфликт» по Организатору): после 409 предлагаем
// ближайшие свободные старты из перезагруженной сетки, начиная с попытки.
// Прошедшие/занятые/хвостовые исключены статусом free — «не раньше сейчас»
// обеспечивает сама сетка. Без имён встреч и записей: приватность (билет #6).
export function nearestFreeStarts(slots: Slot[], attemptedStart: string, limit = 3): Slot[] {
  return slots.filter((s) => s.status === "free" && s.start >= attemptedStart).slice(0, limit);
}

// Источник терминального «Отменена» выводится из состояний, отдельно не
// хранится (GLOSSARY: «Отмена Встречи»): отменённая Встреча — отмена
// Организатора, иначе Запись отменил Посетитель. Для активной Записи — null.
export function cancelSourceLabel(booking: BookingView): string | null {
  if (booking.state !== "cancelled") return null;
  return booking.meetingState === "cancelled"
    ? "отменено организатором"
    : "отменено посетителем";
}

const ruDate = new Intl.DateTimeFormat("ru-RU", {
  weekday: "long",
  day: "numeric",
  month: "long",
});

export function ruLongDate(iso: string): string {
  return ruDate.format(new Date(`${iso}T00:00:00`));
}

export function ruMonthYear(iso: string): string {
  return new Intl.DateTimeFormat("ru-RU", { month: "long", year: "numeric" }).format(
    new Date(`${iso}T00:00:00`),
  );
}

// Сценарий 2: разделение записей на предстоящие и прошедшие.
export function groupBookings<T extends { past: boolean }>(
  items: T[],
): { upcoming: T[]; past: T[] } {
  return {
    upcoming: items.filter((b) => !b.past),
    past: items.filter((b) => b.past),
  };
}
