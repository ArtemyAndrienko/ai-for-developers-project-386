import { describe, expect, it } from "vitest";
import {
  addDays,
  buildWindow,
  groupBookings,
  hmToMin,
  isoDate,
  slotLabel,
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

describe("слоты", () => {
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
