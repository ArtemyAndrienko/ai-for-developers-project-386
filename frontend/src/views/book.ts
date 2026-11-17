// Сценарий 2: посетитель выбирает доступную встречу (она уже назначена на
// конкретный день) и свободный интервал, подтверждает запись имя+email.

import { api, ApiError } from "../api";
import { emailLike, LAST_EMAIL_KEY, ruLongDate, slotLabel, visitorClashes } from "../utils";
import type { Meeting, Slot } from "../types";

type Step = "select" | "confirm" | "done";

interface BookState {
  step: Step;
  meetings: Meeting[];
  meetingId: string | null;
  slots: Slot[];
  slotStart: string | null;
  name: string;
  email: string;
  error: string | null;
}

export async function renderBook(app: HTMLElement): Promise<void> {
  const state: BookState = {
    step: "select",
    meetings: await api.meetings(),
    meetingId: null,
    slots: [],
    slotStart: null,
    name: "",
    email: localStorage.getItem(LAST_EMAIL_KEY) ?? "",
    error: null,
  };

  const meeting = () => state.meetings.find((m) => m.id === state.meetingId) ?? null;

  const ui = () => {
    if (state.meetings.length === 0) {
      app.innerHTML = `<div class="empty-state">Активных встреч пока нет — вернитесь позже.</div>`;
      return;
    }

    if (state.step === "done") {
      app.innerHTML = `
        <div class="cols two">
          ${infoCard(state, meeting())}
          <div class="card center">
            <h2>Бронь подтверждена. До встречи!</h2>
            <button class="btn primary" id="again">Забронировать ещё</button>
            <p class="muted">Все ваши записи — во вкладке «Мои записи».</p>
          </div>
        </div>`;
      app.querySelector("#again")!.addEventListener("click", async () => {
        state.step = "select";
        state.slotStart = null;
        if (state.meetingId) state.slots = await api.slots(state.meetingId);
        ui();
      });
      return;
    }

    if (state.step === "confirm") {
      app.innerHTML = `
        <div class="cols two">
          ${infoCard(state, meeting())}
          <div class="card">
            <div class="card-head"><h3>Подтверждение записи</h3>
              <button class="btn ghost" id="edit">Изменить</button></div>
            ${state.error ? `<div class="error">${state.error}</div>` : ""}
            <label>Имя <input id="name" value="${state.name}" /></label>
            <label>Email <input id="email" type="email" value="${state.email}" /></label>
            <div id="conflict-warn"></div>
            <button class="btn primary block" id="submit">Подтвердить запись</button>
          </div>
        </div>`;
      const checkConflicts = async () => {
        const warn = app.querySelector<HTMLElement>("#conflict-warn")!;
        const m = meeting();
        const slot = state.slots.find((s) => s.start === state.slotStart);
        const email = (app.querySelector("#email") as HTMLInputElement).value.trim();
        if (!m || !slot || !emailLike(email)) {
          warn.innerHTML = "";
          return;
        }
        try {
          const clashes = visitorClashes(
            await api.bookings(email),
            m.id,
            m.date,
            slot.start,
            slot.end,
          );
          warn.innerHTML = clashes
            .map(
              (b) => `<div class="warn">Внимание: у вас уже есть запись ${b.start}–${b.end}
                на «${b.meetingName}» (${b.meetingOwnerName || b.meetingOwnerEmail})
                в это же время. Бронирование не блокируется — но время пересечётся.</div>`,
            )
            .join("");
        } catch {
          warn.innerHTML = ""; // предупреждение — не причина отказывать
        }
      };
      app.querySelector("#email")!.addEventListener("change", () => void checkConflicts());
      void checkConflicts();
      app.querySelector("#edit")!.addEventListener("click", () => {
        state.step = "select";
        state.error = null;
        ui();
      });
      app.querySelector("#submit")!.addEventListener("click", async () => {
        state.name = (app.querySelector("#name") as HTMLInputElement).value.trim();
        state.email = (app.querySelector("#email") as HTMLInputElement).value.trim();
        try {
          await api.book(state.meetingId!, {
            start: state.slotStart!,
            name: state.name,
            email: state.email,
          });
          localStorage.setItem(LAST_EMAIL_KEY, state.email);
          state.step = "done";
        } catch (e) {
          if (e instanceof ApiError && e.status === 409) {
            // Интервал успели забронировать (в т.ч. в другой встрече организатора).
            state.step = "select";
            state.slotStart = null;
            state.error = e.message || "Интервал только что заняли. Выберите другое время.";
            state.slots = await api.slots(state.meetingId!);
          } else {
            state.error = e instanceof Error ? e.message : "Ошибка записи";
          }
        }
        ui();
      });
      return;
    }

    // step === "select"
    app.innerHTML = `
      <div class="meeting-picker">
        ${state.meetings
          .map(
            (m) => `
          <button class="chip${m.id === state.meetingId ? " selected" : ""}" data-meeting="${m.id}">
            <strong>${m.name}</strong>
            <small>${ruLongDate(m.date)} · ${m.durationMin} мин</small>
            <small>${m.description ?? ""}</small>
          </button>`,
          )
          .join("")}
      </div>
      <div class="cols two">
        ${infoCard(state, meeting())}
        <div class="card">
          <h3>Статус интервалов</h3>
          ${
            !state.meetingId
              ? `<div class="hint">Выберите встречу выше.</div>`
              : slotsList(state)
          }
          <div class="row">
            <button class="btn ghost" id="back">Назад</button>
            <button class="btn primary" id="next" ${state.slotStart ? "" : "disabled"}>Продолжить</button>
          </div>
        </div>
      </div>`;

    app.querySelectorAll<HTMLElement>("[data-meeting]").forEach((b) =>
      b.addEventListener("click", async () => {
        state.meetingId = b.dataset.meeting!;
        state.slotStart = null;
        state.error = null;
        state.slots = await api.slots(state.meetingId);
        ui();
      }),
    );

    app.querySelector("#back")!.addEventListener("click", () => {
      state.slotStart = null;
      ui();
    });
    app.querySelector("#next")!.addEventListener("click", () => {
      if (state.slotStart) {
        state.step = "confirm";
        state.error = null;
        ui();
      }
    });

    app.querySelectorAll<HTMLElement>("[data-slot]").forEach((li) =>
      li.addEventListener("click", () => {
        if (li.dataset.selectable === "1") {
          state.slotStart = li.dataset.slot!;
          ui();
        }
      }),
    );
  };

  ui();
}

function infoCard(state: BookState, m: Meeting | null): string {
  const free = state.slots.filter((s) => s.status === "free").length;
  const slot = state.slots.find((s) => s.start === state.slotStart);
  return `
    <div class="card">
      <h3>Информация</h3>
      <div class="kv"><span>Встреча</span><strong>${m ? m.name : "—"}</strong></div>
      <div class="kv"><span>Дата</span><strong>${m ? ruLongDate(m.date) : "Дата не выбрана"}</strong></div>
      <div class="kv"><span>Выбранное время</span><strong>${
        slot ? `${slot.start} - ${slot.end}` : "Время не выбрано"
      }</strong></div>
      <div class="kv"><span>Свободно</span><strong>${state.meetingId ? free : 0}</strong></div>
      <div class="kv"><span>Длительность интервала</span><strong>${
        m ? `${m.durationMin} мин` : "—"
      }</strong></div>
    </div>`;
}

function slotsList(state: BookState): string {
  const parts: string[] = [];
  if (state.error) parts.push(`<div class="error">${state.error}</div>`);
  parts.push(`<ul class="slots">
    ${state.slots
      .map((s) => {
        const selectable = s.status === "free";
        const cls = [
          "slot",
          selectable ? "" : "off",
          state.slotStart === s.start ? "selected" : "",
        ].join(" ");
        return `<li class="${cls}" data-slot="${s.start}" data-selectable="${selectable ? "1" : "0"}">
          <span>${s.start} - ${s.end}</span><b>${slotLabel(s.status)}</b></li>`;
      })
      .join("")}
  </ul>`);
  return parts.join("");
}
