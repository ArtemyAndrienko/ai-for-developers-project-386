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
  tail: "Мало времени",
};

export function slotLabel(status: string): string {
  return slotLabels[status] ?? status;
}

export const emailLike = (s: string) => /\S+@\S+\.\S+/.test(s.trim());

import type { BookingView } from "./types";

// Мягкий конфликт (GLOSSARY: «Конфликт»): пересечение интервала с будущей
// записью посетителя в ДРУГОЙ встрече. Бронированию не мешает — только
// предупреждаем.
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
      !b.past &&
      b.meetingId !== meetingId &&
      b.date === meetingDate &&
      hmToMin(b.start) < e &&
      s < hmToMin(b.end),
  );
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
