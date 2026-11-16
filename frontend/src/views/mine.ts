// Сценарий 2 (финал): все записи посетителя по email; пройдена — в «Прошедшие»,
// которые можно скрыть кнопкой «Скрыть неактуальное».

import { api } from "../api";
import { groupBookings, HIDE_PAST_KEY, LAST_EMAIL_KEY, ruLongDate } from "../utils";
import type { BookingView } from "../types";

export async function renderMine(app: HTMLElement): Promise<void> {
  let hidePast = localStorage.getItem(HIDE_PAST_KEY) === "1";
  let email = localStorage.getItem(LAST_EMAIL_KEY) ?? "";
  let bookings: BookingView[] | null = email ? await fetchBookings(email) : null;

  const ui = () => {
    app.innerHTML = `
      <div class="card">
        <div class="row">
          <label>Email для поиска записей
            <input id="email" type="email" placeholder="you@example.com" value="${email}" />
          </label>
          <button class="btn primary" id="find">Найти</button>
        </div>
      </div>
      ${bookings === null ? `<div class="hint">Введите email, чтобы увидеть свои записи.</div>` : listHtml(bookings, hidePast)}`;

    app.querySelector("#find")!.addEventListener("click", async () => {
      email = (app.querySelector("#email") as HTMLInputElement).value.trim();
      localStorage.setItem(LAST_EMAIL_KEY, email);
      bookings = await fetchBookings(email);
      ui();
    });

    const toggle = app.querySelector("#toggle-past");
    if (toggle) {
      toggle.addEventListener("click", () => {
        hidePast = !hidePast;
        localStorage.setItem(HIDE_PAST_KEY, hidePast ? "1" : "0");
        ui();
      });
    }
  };

  ui();
}

async function fetchBookings(email: string): Promise<BookingView[]> {
  if (!email) return [];
  return api.bookings(email);
}

function listHtml(bookings: BookingView[], hidePast: boolean): string {
  const { upcoming, past } = groupBookings(bookings);
  const organizer = (b: BookingView) =>
    [b.meetingOwnerName, b.meetingOwnerEmail].filter(Boolean).join(" · ");
  const card = (b: BookingView) => `
    <li class="card booking${b.past ? " off" : ""}">
      <div class="booking-head">
        <strong>${b.meetingName || "Встреча удалена"}</strong>
        ${organizer(b) ? `<span class="organizer">Организатор: ${organizer(b)}</span>` : ""}
      </div>
      <div>${ruLongDate(b.date)} · ${b.start} - ${b.end}</div>
      <div class="muted">${b.name}, ${b.email}</div>
      <div class="muted">Создано: ${new Date(b.createdAt).toLocaleString("ru-RU")}</div>
    </li>`;

  return `
    <div class="row between">
      <h2 class="section-title">Предстоящие (${upcoming.length})</h2>
      ${past.length ? `<button class="btn ghost" id="toggle-past">${hidePast ? "Показать неактуальное" : "Скрыть неактуальное"}</button>` : ""}
    </div>
    ${upcoming.length ? `<ul class="booking-list">${upcoming.map(card).join("")}</ul>` : `<div class="empty-state">Предстоящих записей нет.</div>`}
    ${
      past.length && !hidePast
        ? `<h2 class="section-title">Прошедшие (${past.length})</h2>
           <ul class="booking-list">${past.map(card).join("")}</ul>`
        : ""
    }`;
}
