package main

import (
	"errors"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore()
	// Фиксируем «сейчас»: понедельник 16.11.2026, 12:00 локального времени.
	fixed := time.Date(2026, 11, 16, 12, 0, 0, 0, time.Local)
	s.now = func() time.Time { return fixed }
	return s
}

func mustMeeting(t *testing.T, s *Store, name string, dur int, owner, date string) *Meeting {
	t.Helper()
	m, err := s.CreateMeeting(name, "", dur, owner, "", date)
	if err != nil {
		t.Fatalf("CreateMeeting(%q): %v", name, err)
	}
	return m
}

func freeStarts(t *testing.T, s *Store, meetingID string) map[string]SlotStatus {
	t.Helper()
	slots, ok := s.DaySlots(meetingID)
	if !ok {
		t.Fatalf("встреча %s не найдена", meetingID)
	}
	got := map[string]SlotStatus{}
	for _, sl := range slots {
		got[sl.Start] = sl.Status
	}
	return got
}

func TestBook_HappyPath(t *testing.T) {
	s := newTestStore(t)
	m := mustMeeting(t, s, "Звонок", 30, "boss@x.io", "2026-11-17")

	b, err := s.Book(m.ID, "09:00", "Иван", "ivan@example.com")
	if err != nil {
		t.Fatalf("первая бронь: %v", err)
	}
	if b.End != "09:30" || b.Date != "2026-11-17" {
		t.Errorf("бронь наследует дату встречи и окончание = старт + длительность: %+v", b)
	}

	// Второй посетитель на тот же интервал — конфликт.
	if _, err := s.Book(m.ID, "09:00", "Оля", "olya@example.com"); !errors.Is(err, ErrSlotTaken) {
		t.Errorf("повторная бронь занятого слота: %v", err)
	}
	// Следующий интервал — свободен.
	if _, err := s.Book(m.ID, "09:30", "Оля", "olya@example.com"); err != nil {
		t.Errorf("бронь следующего слота: %v", err)
	}
	av, ok := s.MeetingAvailability(m.ID)
	if !ok || av.Date != "2026-11-17" || av.Free != 16 {
		t.Errorf("после двух броней осталось 16 свободных, получили %+v", av)
	}
}

// Занятость владельца сквозная: запись в одну встречу блокирует пересекающиеся
// интервалы в других его встречах на ту же дату.
func TestBook_OwnerBusyAcrossMeetings(t *testing.T) {
	s := newTestStore(t)
	m30 := mustMeeting(t, s, "Короткий колл", 30, "boss@x.io", "2026-11-17")
	m60 := mustMeeting(t, s, "Часовой разбор", 60, "boss@x.io", "2026-11-17")
	mOther := mustMeeting(t, s, "Чужая встреча", 60, "other@x.io", "2026-11-17")

	if _, err := s.Book(m30.ID, "10:30", "Иван", "ivan@example.com"); err != nil {
		t.Fatal(err)
	}

	statuses := freeStarts(t, s, m60.ID)
	if statuses["10:00"] != SlotTaken {
		t.Errorf("слот 10:00–11:00 пересекается с 10:30–11:00 владельца и должен быть занят: %s", statuses["10:00"])
	}
	if statuses["09:00"] != SlotFree || statuses["11:00"] != SlotFree {
		t.Errorf("непересекающиеся слоты должны быть свободны: %v", statuses)
	}
	if _, err := s.Book(m60.ID, "10:00", "Оля", "olya@example.com"); !errors.Is(err, ErrSlotTaken) {
		t.Errorf("бронь пересекающегося слота другой встречи того же владельца: %v", err)
	}

	// Другой владелец в это же время не блокируется.
	if _, err := s.Book(mOther.ID, "10:00", "Оля", "olya@example.com"); err != nil {
		t.Errorf("другой владелец не должен блокироваться: %v", err)
	}

	// Обратное пересечение: после 09:00–10:00 в 60-мин сетке слот 09:00–09:30 занят.
	if _, err := s.Book(m60.ID, "09:00", "Петя", "petya@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Book(m30.ID, "09:00", "Ваня", "vanya@example.com"); !errors.Is(err, ErrSlotTaken) {
		t.Errorf("пересечение обратно тоже должно блокировать: %v", err)
	}
}

// Границы рабочего дня — по почте создателя на конкретную дату встречи;
// дефолт 09:00–18:00; другие даты и другие создатели не затрагиваются.
func TestWorkHours_PerOwnerPerDate(t *testing.T) {
	s := newTestStore(t)

	// Дефолт.
	h := s.WorkHours("boss@x.io", "2026-11-17")
	if h.WorkStart != "09:00" || h.WorkEnd != "18:00" {
		t.Fatalf("дефолтные границы: %+v", h)
	}

	// Сохраняем границы только на 17.11 для boss@x.io.
	if err := s.SetWorkHours("boss@x.io", "2026-11-17", "10:00", "12:00"); err != nil {
		t.Fatal(err)
	}
	m := mustMeeting(t, s, "Узкое окно", 30, "boss@x.io", "2026-11-17")
	slots, _ := s.DaySlots(m.ID)
	if len(slots) != 4 { // 10:00, 10:30, 11:00, 11:30
		t.Fatalf("в сохранённых границах 10:00–12:00 должно быть 4 слота, получили %d", len(slots))
	}
	if slots[0].Start != "10:00" || slots[3].End != "12:00" {
		t.Errorf("сетка не по границам дня: %v … %v", slots[0], slots[3])
	}

	// Та же почта, другая дата — по-прежнему дефолт.
	m2 := mustMeeting(t, s, "Обычный день", 30, "boss@x.io", "2026-11-18")
	if slots2, _ := s.DaySlots(m2.ID); len(slots2) != 18 {
		t.Errorf("на 18.11 границы не должны были измениться, слотов %d", len(slots2))
	}
	// Другой создатель на 17.11 — дефолт.
	if h := s.WorkHours("other@x.io", "2026-11-17"); h.WorkStart != "09:00" {
		t.Errorf("границы не должны протекать между создателями: %+v", h)
	}

	// Валидация.
	if err := s.SetWorkHours("boss@x.io", "2026-11-17", "18:00", "09:00"); err == nil {
		t.Error("окончание раньше начала должно отклоняться")
	}
	if err := s.SetWorkHours("not-an-email", "2026-11-17", "10:00", "12:00"); !errors.Is(err, ErrOwnerRequired) {
		t.Errorf("email создателя обязателен: %v", err)
	}
	if err := s.SetWorkHours("boss@x.io", "2026-13-45", "10:00", "12:00"); err == nil {
		t.Error("некорректная дата должна отклоняться")
	}
}

// Превью создаваемой встречи: границы для (owner, date) + занятость владельца.
func TestPreviewSlots(t *testing.T) {
	s := newTestStore(t)
	m30 := mustMeeting(t, s, "Звонок", 30, "boss@x.io", "2026-11-17")
	if _, err := s.Book(m30.ID, "10:30", "Иван", "ivan@example.com"); err != nil {
		t.Fatal(err)
	}

	slots := s.PreviewSlots("boss@x.io", "2026-11-17", 60)
	found := false
	for _, sl := range slots {
		if sl.Start == "10:00" {
			found = true
			if sl.Status != SlotTaken {
				t.Errorf("в превью слот 10:00 должен быть занят, статус %s", sl.Status)
			}
		}
	}
	if !found {
		t.Fatal("в превью нет слота 10:00")
	}

	// Без owner превью не учитывает чужие брони.
	for _, sl := range s.PreviewSlots("", "2026-11-17", 60) {
		if sl.Start == "10:00" && sl.Status != SlotFree {
			t.Error("без email создателя превью не должно показывать занятость")
		}
	}

	// Границы конкретной даты применяются и в превью.
	if err := s.SetWorkHours("boss@x.io", "2026-11-19", "14:00", "18:00"); err != nil {
		t.Fatal(err)
	}
	pv := s.PreviewSlots("boss@x.io", "2026-11-19", 30)
	if pv[0].Start != "14:00" {
		t.Errorf("превью должно использовать границы 14:00–18:00, первый слот %s", pv[0].Start)
	}
}

func TestCreateMeeting_Validation(t *testing.T) {
	s := newTestStore(t)
	cases := []struct {
		name       string
		mname, own string
		dur        int
		date       string
		wantErr    error
	}{
		{"без названия", "", "boss@x.io", 30, "2026-11-17", nil},
		{"нулевая длительность", "x", "boss@x.io", 0, "2026-11-17", nil},
		{"без email создателя", "x", "", 30, "2026-11-17", ErrOwnerRequired},
		{"некорректный email", "x", "nope", 30, "2026-11-17", ErrOwnerRequired},
		{"дата в прошлом", "x", "boss@x.io", 30, "2026-11-10", ErrDateRequired},
		{"дата вне окна", "x", "boss@x.io", 30, "2026-12-31", ErrDateRequired},
		{"кривая дата", "x", "boss@x.io", 30, "17.11.2026", ErrDateRequired},
	}
	for _, c := range cases {
		_, err := s.CreateMeeting(c.mname, "", c.dur, c.own, "", c.date)
		if err == nil {
			t.Errorf("%s: ошибки нет", c.name)
		}
		if c.wantErr != nil && !errors.Is(err, c.wantErr) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.wantErr)
		}
	}
}

func TestMeetingsFilterByOwner(t *testing.T) {
	s := newTestStore(t)
	mustMeeting(t, s, "Поздняя", 30, "boss@x.io", "2026-11-20")
	mustMeeting(t, s, "Ранняя", 30, "BOSS@X.IO", "2026-11-17") // email нечувствителен к регистру
	mustMeeting(t, s, "Чужая", 30, "other@x.io", "2026-11-18")
	got := s.Meetings("boss@x.io")
	if len(got) != 2 || got[0].Name != "Ранняя" || got[1].Name != "Поздняя" {
		t.Errorf("фильтр/сортировка по владельцу: %+v", got)
	}
	if all := s.Meetings(""); len(all) != 3 {
		t.Errorf("без фильтра — все встречи: %d", len(all))
	}
}

// Организатор: имя необязательное, хранится вместе со встречей и
// отображается посетителю в «Мои записи».
func TestMeetingOrganizerName(t *testing.T) {
	s := newTestStore(t)
	m, err := s.CreateMeeting("Разбор", "", 30, "boss@x.io", "Иван Босс", "2026-11-17")
	if err != nil {
		t.Fatal(err)
	}
	if m.OwnerName != "Иван Босс" || m.OwnerEmail != "boss@x.io" {
		t.Errorf("имя организатора не сохранилось: %+v", m)
	}
	// Без имени — пустая строка допустима.
	m2, err := s.CreateMeeting("Аноним", "", 30, "boss@x.io", "", "2026-11-18")
	if err != nil || m2.OwnerName != "" {
		t.Errorf("имя организатора опционально: %+v %v", m2, err)
	}
}

func TestPreviewAndDeleteMeeting(t *testing.T) {
	s := newTestStore(t)
	avail := s.PreviewAvailability(30, "")
	if len(avail) != windowDays || avail[0].Date != "2026-11-16" || avail[13].Date != "2026-11-29" {
		t.Fatalf("окно превью: %+v", avail)
	}
	if avail[0].Free != 11 { // сегодня: сейчас 12:00 → слоты с 12:30 по 17:30
		t.Errorf("на сегодня 11 свободных, получили %d", avail[0].Free)
	}

	m := mustMeeting(t, s, "Звонок", 30, "boss@x.io", "2026-11-17")
	if _, err := s.Book(m.ID, "09:00", "Иван", "ivan@example.com"); err != nil {
		t.Fatal(err)
	}
	s.DeleteMeeting(m.ID)
	if bs := s.BookingsByUser("ivan@example.com"); len(bs) != 0 {
		t.Errorf("после отмены встречи записей быть не должно: %d", len(bs))
	}
}

func TestBook_Rejections(t *testing.T) {
	s := newTestStore(t)
	today := mustMeeting(t, s, "Сегодня", 30, "boss@x.io", "2026-11-16")
	future := mustMeeting(t, s, "Будущая", 30, "boss@x.io", "2026-11-17")

	if _, err := s.Book(today.ID, "10:00", "Иван", "ivan@x.io"); !errors.Is(err, ErrSlotUnavailable) {
		t.Errorf("прошедший интервал сегодняшнего дня: %v", err)
	}
	if _, err := s.Book(future.ID, "09:15", "Иван", "ivan@x.io"); !errors.Is(err, ErrSlotUnavailable) {
		t.Errorf("интервал вне сетки: %v", err)
	}
	if _, err := s.Book(future.ID, "09:00", "", "ivan@x.io"); !errors.Is(err, ErrNameRequired) {
		t.Errorf("пустое имя: %v", err)
	}
	if _, err := s.Book(future.ID, "09:00", "Иван", "nope"); !errors.Is(err, ErrEmailRequired) {
		t.Errorf("некорректный email посетителя: %v", err)
	}
	if _, err := s.Book("m-nope", "09:00", "n", "e@x.io"); !errors.Is(err, ErrMeetingNotFound) {
		t.Errorf("несуществующая встреча: %v", err)
	}
	// Встреча задним числом с датой вне окна (данные «устарели» в памяти).
	stale := &Meeting{ID: "m-stale", Name: "Просроченная", DurationMin: 30, OwnerEmail: "boss@x.io", Date: "2026-11-10"}
	s.mu.Lock()
	s.meetings[stale.ID] = stale
	s.mu.Unlock()
	if _, err := s.Book(stale.ID, "09:00", "Иван", "ivan@x.io"); !errors.Is(err, ErrOutsideWindow) {
		t.Errorf("бронь встречи вне окна: %v", err)
	}
}

// Хвостовой остаток на день встречи нельзя забронировать.
func TestBook_TailSlotRejected(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetWorkHours("boss@x.io", "2026-11-17", "09:00", "17:50"); err != nil {
		t.Fatal(err)
	}
	m := mustMeeting(t, s, "Долгий созвон", 45, "boss@x.io", "2026-11-17")
	slots, _ := s.DaySlots(m.ID)
	tail := slots[len(slots)-1]
	if tail.Status != SlotTail {
		t.Fatalf("последний интервал должен быть серым: %v", tail)
	}
	if _, err := s.Book(m.ID, tail.Start, "Иван", "ivan@example.com"); !errors.Is(err, ErrSlotUnavailable) {
		t.Errorf("бронь хвостового слота: %v", err)
	}
}

// Сценарий 2: прошедшая встреча попадает в «Прошедшие», будущая — нет.
func TestIsPast_Grouping(t *testing.T) {
	s := newTestStore(t)
	m := mustMeeting(t, s, "Звонок", 30, "boss@x.io", "2026-11-17")
	future, err := s.Book(m.ID, "09:00", "Иван", "ivan@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if s.IsPast(future) {
		t.Errorf("будущая встреча не должна быть past")
	}
	pastB := &Booking{ID: "b-old", MeetingID: m.ID, Date: "2026-11-10", Start: "09:00", End: "09:30", Email: "ivan@example.com", Name: "Иван"}
	s.mu.Lock()
	s.bookings[pastB.ID] = pastB
	s.mu.Unlock()
	if !s.IsPast(pastB) {
		t.Errorf("встреча 10.11 должна быть прошедшей")
	}
	bs := s.BookingsByUser("ivan@example.com")
	if len(bs) != 2 || bs[0].Date != "2026-11-10" {
		t.Errorf("сортировка записей по времени: %+v", bs)
	}
}
