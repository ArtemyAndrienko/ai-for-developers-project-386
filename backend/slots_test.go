package main

import (
	"testing"
)

// Полный рабочий день 09:00–18:00 с шагом 30 минут даёт ровно 18 интервалов
// и без хвостового остатка (референс: бейдж «18 св.»).
func TestGenerateSlots_FullDayNoTail(t *testing.T) {
	slots := GenerateSlots("09:00", "18:00", 30, DayInFuture, nil)
	if len(slots) != 18 {
		t.Fatalf("ожидалось 18 интервалов, получено %d", len(slots))
	}
	if slots[0].Start != "09:00" || slots[0].End != "09:30" {
		t.Errorf("первый интервал %v", slots[0])
	}
	if last := slots[17]; last.Start != "17:30" || last.End != "18:00" {
		t.Errorf("последний интервал %v", last)
	}
	for _, s := range slots {
		if s.Status != SlotFree {
			t.Errorf("интервал %v должен быть свободен", s)
		}
	}
}

// Остаток дня короче продолжительности отмечается серым (SlotTail):
// 09:00–17:50 с шагом 45 минут — последние 35 минут (17:15–17:50) не
// дотягивают до 45, это серый хвост.
func TestGenerateSlots_TailIsGray(t *testing.T) {
	full := GenerateSlots("09:00", "17:50", 45, DayInFuture, nil)
	if len(full) == 0 {
		t.Fatal("пустой список")
	}
	last := full[len(full)-1]
	if last.Status != SlotTail {
		t.Fatalf("хвостовой интервал должен быть серым, получили %v", last)
	}
	if last.Start != "17:15" || last.End != "17:50" {
		t.Errorf("хвостовой интервал: %v", last)
	}
	if FreeCount(full) != len(full)-1 {
		t.Errorf("хвостовой интервал не должен считаться свободным")
	}
}

func TestGenerateSlots_Statuses(t *testing.T) {
	busy := []Interval{{Start: 9 * 60, End: 9*60 + 30}, {Start: 9*60 + 30, End: 10 * 60}}
	// now = 10:00: интервалы с началом <= 10:00 — прошедшие (если не заняты).
	slots := GenerateSlots("09:00", "12:00", 30, 10*60, busy)
	want := map[string]SlotStatus{
		"09:00": SlotTaken, // занят, приоритет над прошедшим
		"09:30": SlotTaken,
		"10:00": SlotPast,
		"10:30": SlotFree,
		"11:00": SlotFree,
		"11:30": SlotFree,
	}
	if len(slots) != len(want) {
		t.Fatalf("ожидалось %d интервалов, получено %d", len(want), len(slots))
	}
	for _, s := range slots {
		if want[s.Start] != s.Status {
			t.Errorf("интервал %s: статус %s, ожидался %s", s.Start, s.Status, want[s.Start])
		}
	}
}

// Занятость из другой встречи: 60-минутный интервал 10:00–11:00 пересекается с
// уже забронированным 10:30–11:00 (30-мин.) — помечается занятым.
func TestGenerateSlots_OverlapFromOtherMeeting(t *testing.T) {
	busy := []Interval{{Start: 10*60 + 30, End: 11 * 60}}
	slots := GenerateSlots("09:00", "12:00", 60, DayInFuture, busy)
	want := map[string]SlotStatus{
		"09:00": SlotFree, // 09:00–10:00 — пересечения нет
		"10:00": SlotTaken,
		"11:00": SlotFree,
	}
	if len(slots) != 3 {
		t.Fatalf("3 шестидесятиминутных интервала, получено %d", len(slots))
	}
	for _, s := range slots {
		if want[s.Start] != s.Status {
			t.Errorf("интервал %s: статус %s, ожидался %s", s.Start, s.Status, want[s.Start])
		}
	}
}

func TestGenerateSlots_DayInPast(t *testing.T) {
	slots := GenerateSlots("09:00", "18:00", 30, DayInPast, nil)
	for _, s := range slots {
		if s.Status != SlotPast {
			t.Fatalf("вчерашний день целиком должен быть past: %v", s)
		}
	}
}

func TestSlotAllowed(t *testing.T) {
	slots := GenerateSlots("09:00", "18:00", 30, DayInFuture, nil)
	if _, ok := SlotAllowed(slots, "09:00"); !ok {
		t.Errorf("свободный интервал должен разрешать бронь")
	}
	if _, ok := SlotAllowed(slots, "09:15"); ok {
		t.Errorf("интервала вне сетки не существует")
	}
	tailDay := GenerateSlots("09:00", "17:50", 45, DayInFuture, nil)
	tail := tailDay[len(tailDay)-1]
	if _, ok := SlotAllowed(tailDay, tail.Start); ok {
		t.Errorf("хвостовой серый интервал нельзя бронировать")
	}
}

func TestParseHM(t *testing.T) {
	for _, s := range []string{"00:00", "09:05", "18:00", "23:59"} {
		m, ok := ParseHM(s)
		if !ok {
			t.Fatalf("ParseHM(%q) не разобрался", s)
		}
		if got := FormatHM(m); got != s {
			t.Errorf("FormatHM(ParseHM(%q)) = %q", s, got)
		}
	}
	for _, bad := range []string{"9:0", "24:00", "09:60", "", "abc", "090-00"} {
		if _, ok := ParseHM(bad); ok {
			t.Errorf("ParseHM(%q) должен отказать", bad)
		}
	}
}

func TestValidEmail(t *testing.T) {
	for _, okCase := range []string{"a@b.io", "ivan+test@example.com"} {
		if !ValidEmail(okCase) {
			t.Errorf("%q должна проходить", okCase)
		}
	}
	for _, bad := range []string{"", "a", "@b.io", "a@", "a b@x.io"} {
		if ValidEmail(bad) {
			t.Errorf("%q должна отклоняться", bad)
		}
	}
}
