import { describe, expect, it } from "vitest";
import type { BookingView } from "./types";
import {
  addDays,
  buildWindow,
  emailLike,
  groupBookings,
  hmToMin,
  isoDate,
  slotLabel,
  visitorClashes,
  weekStartMonday,
} from "./utils";

describe("даты", () => {
  it("addDays переносит через границу месяца и года", () => {
    expect(addDays("2026-11-16", 1)).toBe("2026-11-17");
    expect(addDays("2026-11-30", 1)).toBe("2026-12-01");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2026-11-16", -1)).toBe("2026-11-15");
  });

  it("isoDate форматирует локальную дату", () => {
    expect(isoDate(new Date(2026, 10, 6))).toBe("2026-11-06");
  });

  it("weekStartMonday даёт понедельник", () => {
    // 16.11.2026 — понедельник
    expect(weekStartMonday("2026-11-16")).toBe("2026-11-16");
    expect(weekStartMonday("2026-11-22")).toBe("2026-11-16"); // воскресенье
    expect(weekStartMonday("2026-11-23")).toBe("2026-11-23"); // понедельник
  });

  it("окно доступности — 14 дней от сегодня включительно", () => {
    const w = buildWindow("2026-11-16");
    expect(w).toHaveLength(14);
    expect(w[0]).toBe("2026-11-16");
    expect(w[13]).toBe("2026-11-29");
  });
});

describe("интервалы", () => {
  it("hmToMin переводит HH:MM в минуты", () => {
    expect(hmToMin("09:00")).toBe(540);
    expect(hmToMin("17:30")).toBe(1050);
  });

  it("метки статусов на русском, как в референсе", () => {
    expect(slotLabel("free")).toBe("Свободно");
    expect(slotLabel("taken")).toBe("Занято");
    expect(slotLabel("tail")).toBe("Мало времени");
  });
});

describe("группировка записей (Сценарий 2)", () => {
  it("прошедшие отделяются от предстоящих", () => {
    const { upcoming, past } = groupBookings([
      { past: false },
      { past: true },
      { past: false },
    ]);
    expect(upcoming).toHaveLength(2);
    expect(past).toHaveLength(1);
  });

  it("пустой список — пустые группы (если встреч нет, список пустой)", () => {
    expect(groupBookings([])).toEqual({ upcoming: [], past: [] });
  });
});

describe("мягкий конфликт посетителя (GLOSSARY: Конфликт)", () => {
  const other = (over: Record<string, unknown>) =>
    ({
      past: false,
      meetingId: "m-9",
      date: "2026-11-20",
      start: "10:00",
      end: "10:30",
      ...over,
    }) as BookingView;

  it("пересекающаяся будущая запись в другой встрече — конфликт", () => {
    const clashes = visitorClashes([other({ start: "10:15", end: "11:00" })], "m-1", "2026-11-20", "10:00", "10:30");
    expect(clashes).toHaveLength(1);
  });

  it("касание границ (конец = начало) — не конфликт", () => {
    expect(visitorClashes([other({ start: "09:30", end: "10:00" })], "m-1", "2026-11-20", "10:00", "10:30")).toHaveLength(0);
  });

  it("та же встреча, другой день и прошедшая запись — не конфликт", () => {
    expect(visitorClashes([other({ meetingId: "m-1" })], "m-1", "2026-11-20", "10:00", "10:30")).toHaveLength(0);
    expect(visitorClashes([other({ date: "2026-11-21" })], "m-1", "2026-11-20", "10:00", "10:30")).toHaveLength(0);
    expect(visitorClashes([other({ past: true })], "m-1", "2026-11-20", "10:00", "10:30")).toHaveLength(0);
  });
});

describe("emailLike", () => {
  it("принимает корректные и отвергает мусор", () => {
    expect(emailLike("a@b.io")).toBe(true);
    expect(emailLike("boss@x.io")).toBe(true);
    expect(emailLike("boss@x")).toBe(false); // требует доменной точки — мягче сервера не надо
    expect(emailLike("nope")).toBe(false);
    expect(emailLike("")).toBe(false);
  });
});
