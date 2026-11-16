package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Hours — границы рабочего дня. Хранятся для конкретной пары
// (email создателя, дата): «границы применяются на конкретный день,
// на который назначена встреча, и сопоставляются по адресу почты».
// Значение по умолчанию — 09:00–18:00.
type Hours struct {
	WorkStart string `json:"workStart"` // "HH:MM"
	WorkEnd   string `json:"workEnd"`
}

func defaultHours() Hours { return Hours{WorkStart: "09:00", WorkEnd: "18:00"} }

type Meeting struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DurationMin int       `json:"durationMin"` // продолжительность интервала
	OwnerEmail  string    `json:"ownerEmail"`  // почта создателя: по ней проверяется занятость
	OwnerName   string    `json:"ownerName"`   // имя организатора (необязательное, для отображения)
	Date        string    `json:"date"`        // день, на который назначена встреча "2006-01-02"
	CreatedAt   time.Time `json:"createdAt"`
}

type Booking struct {
	ID        string    `json:"id"`
	MeetingID string    `json:"meetingId"`
	Date      string    `json:"date"`  // "2026-11-20"
	Start     string    `json:"start"` // "10:30"
	End       string    `json:"end"`   // "11:00"
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type DayAvailability struct {
	Date string `json:"date"`
	Free int    `json:"free"` // число свободных интервалов (бейдж «18 св.»)
}

// Ошибки различимы на уровне HTTP-кодов (Сценарий 2: конфликты).
var (
	ErrMeetingNotFound = fmt.Errorf("встреча не найдена")
	ErrSlotUnavailable = fmt.Errorf("слот недоступен (серый или прошедший)")
	ErrSlotTaken       = fmt.Errorf("слот занят в этой или другой встрече владельца")
	ErrNameRequired    = fmt.Errorf("укажите имя")
	ErrEmailRequired   = fmt.Errorf("укажите email")
	ErrOwnerRequired   = fmt.Errorf("укажите email создателя")
	ErrDateRequired    = fmt.Errorf("дата встречи обязательна и должна быть в окне доступности")
	ErrOutsideWindow   = fmt.Errorf("дата вне окна доступности")
)

// Store — внутреннее хранилище на Go-структурах данных.
// Заменится на embedded-хранилище, методы останутся теми же.
type Store struct {
	mu       sync.RWMutex
	hours    map[string]map[string]Hours // owner email -> date -> границы дня
	meetings map[string]*Meeting
	bookings map[string]*Booking
	seq      int64
	now      func() time.Time // подменяется в тестах
}

func NewStore() *Store {
	return &Store{
		hours:    map[string]map[string]Hours{},
		meetings: map[string]*Meeting{},
		bookings: map[string]*Booking{},
		now:      time.Now,
	}
}

func (s *Store) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s-%d", prefix, s.seq)
}

func (s *Store) today() string { return s.now().Format("2006-01-02") }

func (s *Store) windowLast() string {
	base, _ := time.Parse("2006-01-02", s.today())
	return base.AddDate(0, 0, windowDays-1).Format("2006-01-02")
}

func (s *Store) inWindow(date string) bool {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return false
	}
	today := s.today()
	return date >= today && date <= s.windowLast()
}

// --- Границы рабочего дня: по почте создателя на конкретную дату ---

// hoursAt — границы дня без захвата блокировки; вызывать только владея s.mu.
func (s *Store) hoursAt(owner, date string) Hours {
	if byDate, ok := s.hours[owner]; ok {
		if h, ok := byDate[date]; ok {
			return h
		}
	}
	return defaultHours()
}

// WorkHours возвращает границы дня для пары (owner, date); если сохранённых
// нет — дефолт 09:00–18:00.
func (s *Store) WorkHours(owner, date string) Hours {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hoursAt(owner, date)
}

// SetWorkHours сохраняет границы только для одной даты одного создателя —
// остальные дни и другие пользователи не затрагиваются.
func (s *Store) SetWorkHours(owner, date, workStart, workEnd string) error {
	if !ValidEmail(owner) {
		return ErrOwnerRequired
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("некорректная дата: %s", date)
	}
	st, ok1 := ParseHM(workStart)
	en, ok2 := ParseHM(workEnd)
	if !ok1 || !ok2 || en <= st {
		return fmt.Errorf("некорректные границы рабочего дня: %s / %s", workStart, workEnd)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hours[owner] == nil {
		s.hours[owner] = map[string]Hours{}
	}
	s.hours[owner][date] = Hours{WorkStart: FormatHM(st), WorkEnd: FormatHM(en)}
	return nil
}

// --- Встречи ---

// CreateMeeting создаёт встречу, назначенную на конкретный день окна.
func (s *Store) CreateMeeting(name, description string, durationMin int, ownerEmail, ownerName, date string) (*Meeting, error) {
	if name == "" {
		return nil, fmt.Errorf("название обязательно")
	}
	if !ValidEmail(ownerEmail) {
		return nil, ErrOwnerRequired
	}
	if !s.inWindow(date) {
		return nil, ErrDateRequired
	}
	if durationMin <= 0 || durationMin > 24*60 {
		return nil, fmt.Errorf("некорректная продолжительность: %d", durationMin)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &Meeting{
		ID:          s.nextID("m"),
		Name:        name,
		Description: description,
		DurationMin: durationMin,
		OwnerEmail:  ownerEmail,
		OwnerName:   ownerName,
		Date:        date,
		CreatedAt:   s.now().UTC(),
	}
	s.meetings[m.ID] = m
	return m, nil
}

// Meetings возвращает все встречи (для посетителя) или встречи одного
// создателя, если owner непустой (Сценарий 1: «список всех встреч»).
func (s *Store) Meetings(owner string) []*Meeting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Meeting, 0, len(s.meetings))
	for _, m := range s.meetings {
		if owner == "" || strings.EqualFold(m.OwnerEmail, owner) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Store) Meeting(id string) (*Meeting, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.meetings[id]
	return m, ok
}

// DeleteMeeting отменяет встречу: её интервалы снова свободны,
// записи посетителей удаляются.
func (s *Store) DeleteMeeting(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.meetings, id)
	for bID, b := range s.bookings {
		if b.MeetingID == id {
			delete(s.bookings, bID)
		}
	}
}

// --- Занятость владельца ---

// ownerBusy возвращает занятые отрезки дня во всех встречах владельца
// (email) на указанную дату. Бронируя интервал одной встречи, посетитель
// занимает время владельца — в другой его встрече этот отрезок уже нельзя
// выбрать, даже если сетки интервалов не совпадают (сравнение пересечений).
// Вызывать под блокировкой s.mu.
func (s *Store) ownerBusy(ownerEmail, date string) []Interval {
	busy := make([]Interval, 0)
	ownerMeetings := map[string]bool{}
	for _, m := range s.meetings {
		if strings.EqualFold(m.OwnerEmail, ownerEmail) {
			ownerMeetings[m.ID] = true
		}
	}
	for _, b := range s.bookings {
		if !ownerMeetings[b.MeetingID] || b.Date != date {
			continue
		}
		st, ok1 := ParseHM(b.Start)
		en, ok2 := ParseHM(b.End)
		if ok1 && ok2 {
			busy = append(busy, Interval{Start: st, End: en})
		}
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start < busy[j].Start })
	return busy
}

// --- Интервалы дня ---

// dayNowMin возвращает «сейчас» в минутах для выбранного дня: дни в будущем и
// прошлом получают специальные значения, чтобы ни один слот не считался
// прошедшим / все считались прошедшими.
func (s *Store) dayNowMin(date string) int {
	now := s.now()
	today := now.Format("2006-01-02")
	switch {
	case date > today:
		return DayInFuture
	case date < today:
		return DayInPast
	default:
		return now.Hour()*60 + now.Minute()
	}
}

// DaySlots — сетка интервалов встречи на её день: границы рабочего дня
// берутся для (owner, date), занятость — со сквозными пересечениями по
// всем встречам владельца.
func (s *Store) DaySlots(meetingID string) ([]Slot, bool) {
	m, ok := s.Meeting(meetingID)
	if !ok {
		return nil, false
	}
	h := s.WorkHours(m.OwnerEmail, m.Date)
	s.mu.RLock()
	busy := s.ownerBusy(m.OwnerEmail, m.Date)
	s.mu.RUnlock()
	return GenerateSlots(h.WorkStart, h.WorkEnd, m.DurationMin, s.dayNowMin(m.Date), busy), true
}

// MeetingAvailability — свободные интервалы на день встречи (для карточек).
func (s *Store) MeetingAvailability(meetingID string) (DayAvailability, bool) {
	slots, ok := s.DaySlots(meetingID)
	if !ok {
		return DayAvailability{}, false
	}
	m, _ := s.Meeting(meetingID)
	return DayAvailability{Date: m.Date, Free: FreeCount(slots)}, true
}

// PreviewSlots — превью сетки для ещё не созданной встречи (Сценарий 1):
// границы для (owner, date), занятость владельца на эту дату.
func (s *Store) PreviewSlots(ownerEmail, date string, durationMin int) []Slot {
	h := s.WorkHours(ownerEmail, date)
	var busy []Interval
	if ownerEmail != "" {
		s.mu.RLock()
		busy = s.ownerBusy(ownerEmail, date)
		s.mu.RUnlock()
	}
	return GenerateSlots(h.WorkStart, h.WorkEnd, durationMin, s.dayNowMin(date), busy)
}

// PreviewAvailability — окно доступности для создаваемой встречи: каждый день
// считается по своим границам для этого владельца.
func (s *Store) PreviewAvailability(durationMin int, ownerEmail string) []DayAvailability {
	out := make([]DayAvailability, 0, windowDays)
	base, _ := time.Parse("2006-01-02", s.today())
	for i := 0; i < windowDays; i++ {
		date := base.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, DayAvailability{Date: date, Free: FreeCount(s.PreviewSlots(ownerEmail, date, durationMin))})
	}
	return out
}

// --- Бронирование ---

// Book занимает интервал встречи; день берётся из самой встречи.
func (s *Store) Book(meetingID, start, name, email string) (*Booking, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if !ValidEmail(email) {
		return nil, ErrEmailRequired
	}
	m, ok := s.Meeting(meetingID)
	if !ok {
		return nil, ErrMeetingNotFound
	}
	if !s.inWindow(m.Date) { // защита от данных, устаревших в памяти
		return nil, ErrOutsideWindow
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.hoursAt(m.OwnerEmail, m.Date)
	slots := GenerateSlots(h.WorkStart, h.WorkEnd, m.DurationMin, s.dayNowMin(m.Date), s.ownerBusy(m.OwnerEmail, m.Date))
	slot, allowed := SlotAllowed(slots, start)
	if !allowed {
		if slot.Status == SlotTaken {
			return nil, ErrSlotTaken
		}
		return nil, ErrSlotUnavailable
	}
	b := &Booking{
		ID:        s.nextID("b"),
		MeetingID: meetingID,
		Date:      m.Date,
		Start:     start,
		End:       slot.End,
		Name:      name,
		Email:     email,
		CreatedAt: s.now().UTC(),
	}
	s.bookings[b.ID] = b
	return b, nil
}

// BookingsByUser — все записи посетителя по email, отсортированы по времени.
func (s *Store) BookingsByUser(email string) []*Booking {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Booking, 0)
	for _, b := range s.bookings {
		if strings.EqualFold(b.Email, email) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Start < out[j].Start
	})
	return out
}

// IsPast — встреча закончилась (Сценарий 2: автоперенос в «Прошедшие»).
func (s *Store) IsPast(b *Booking) bool {
	now := s.now()
	endMin, ok := ParseHM(b.End)
	if !ok {
		return true
	}
	d, err := time.Parse("2006-01-02", b.Date)
	if err != nil {
		return true
	}
	end := time.Date(d.Year(), d.Month(), d.Day(), 0, endMin, 0, 0, now.Location())
	return !end.After(now)
}
