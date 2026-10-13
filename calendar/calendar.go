// Package calendar renders an inline date picker: a month grid with
// previous/next navigation, stateless like the rest of gogram's keyboards.
//
//	cal := calendar.New("cal", calendar.WeekStart(time.Monday), calendar.Min(time.Now()))
//	cal.Handle(router, func(ctx *gogram.Context, _ *gogram.CallbackQuery, date time.Time) error {
//		return ctx.EditMessageText("Booked " + date.Format("2006-01-02"))
//	})
//
//	router.HandleCommand("/book", func(ctx *gogram.Context, _ *gogram.Message) error {
//		now := time.Now()
//		kb := cal.Keyboard(now.Year(), now.Month())
//
//		return ctx.SendMessage("Pick a date", gogram.WithSendMessageReplyMarkup(kb.ReplyMarkup()))
//	})
package calendar

import (
	"strconv"
	"time"

	"github.com/darxnet/gogram"
)

// month is the navigation callback data: show this month.
type month struct {
	Y int
	M int
}

// day is the pick callback data.
type day struct {
	Y int
	M int
	D int
}

// Option configures a [Calendar].
type Option func(*Calendar)

// WeekStart sets the first day of the week. Default: Monday.
func WeekStart(d time.Weekday) Option { return func(c *Calendar) { c.weekStart = d } }

// Names sets localized names: 12 months and 7 weekdays starting from
// Sunday, as in [time.Weekday]. Default: English. It panics on other lengths.
func Names(months, weekdays []string) Option {
	if len(months) != 12 || len(weekdays) != 7 {
		panic("calendar: Names needs 12 month and 7 weekday names")
	}

	return func(c *Calendar) {
		copy(c.months[:], months)
		copy(c.weekdays[:], weekdays)
	}
}

// Min disables days before t (the date part of t, in its location).
func Min(t time.Time) Option { return func(c *Calendar) { c.min = dateOf(t) } }

// Max disables days after t.
func Max(t time.Time) Option { return func(c *Calendar) { c.max = dateOf(t) } }

// Disabled disables days for which f reports true, e.g. weekends or booked slots.
func Disabled(f func(time.Time) bool) Option { return func(c *Calendar) { c.disabled = f } }

// Location sets the location picked dates are expressed in. Default: UTC.
func Location(loc *time.Location) Option { return func(c *Calendar) { c.loc = loc } }

// PickHandler receives the picked date, at midnight in the calendar's location.
type PickHandler func(ctx *gogram.Context, cq *gogram.CallbackQuery, date time.Time) error

// Calendar renders month grids and decodes picks.
type Calendar struct {
	nav  *gogram.Callback[month]
	pick *gogram.Callback[day]
	noop gogram.InlineKeyboardButton

	weekStart time.Weekday
	months    [12]string
	weekdays  [7]string
	min, max  time.Time // zero when unset
	disabled  func(time.Time) bool
	loc       *time.Location
}

// New creates a calendar whose callbacks start with prefix.
func New(prefix string, opts ...Option) *Calendar {
	c := &Calendar{
		nav:       gogram.NewCallback[month](prefix + "m"),
		pick:      gogram.NewCallback[day](prefix + "d"),
		noop:      gogram.InlineKeyboardButton{Text: " ", CallbackData: prefix + "x"},
		weekStart: time.Monday,
		months: [12]string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
		weekdays: [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
		loc:      time.UTC,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Keyboard renders the grid of year/month: a header row with navigation,
// the weekday row and the weeks. Disabled days are blank buttons.
func (c *Calendar) Keyboard(year int, m time.Month) *gogram.InlineKeyboardBuilder {
	first := time.Date(year, m, 1, 0, 0, 0, 0, c.loc)
	prev := first.AddDate(0, -1, 0)
	next := first.AddDate(0, 1, 0)

	kb := gogram.NewInlineKeyboardBuilder().Add(
		c.nav.Button("‹", month{Y: prev.Year(), M: int(prev.Month())}),
		gogram.InlineKeyboardButton{Text: c.months[m-1] + " " + strconv.Itoa(year), CallbackData: c.noop.CallbackData},
		c.nav.Button("›", month{Y: next.Year(), M: int(next.Month())}),
	).Row()

	for i := range 7 {
		kb.Add(gogram.InlineKeyboardButton{
			Text:         c.weekdays[(int(c.weekStart)+i)%7],
			CallbackData: c.noop.CallbackData,
		})
	}

	kb.Row()

	// Leading blanks up to the weekday of the 1st.
	lead := (int(first.Weekday()) - int(c.weekStart) + 7) % 7
	for range lead {
		kb.Add(c.noop)
	}

	cells := lead

	for d := first; d.Month() == m; d = d.AddDate(0, 0, 1) {
		if c.isDisabled(d) {
			kb.Add(c.noop)
		} else {
			kb.Add(c.pick.Button(strconv.Itoa(d.Day()), day{Y: year, M: int(m), D: d.Day()}))
		}

		cells++

		if cells%7 == 0 {
			kb.Row()
		}
	}

	// Trailing blanks to a full week.
	for ; cells%7 != 0; cells++ {
		kb.Add(c.noop)
	}

	return kb.Row()
}

// Handle registers the calendar's callbacks on rg: navigation re-renders
// the message's keyboard, a pick calls onPick with the date in the
// calendar's location, blank cells only get answered.
func (c *Calendar) Handle(rg *gogram.RouterGroup, onPick PickHandler) {
	c.nav.Handle(rg, func(ctx *gogram.Context, _ *gogram.CallbackQuery, m month) error {
		kb := c.Keyboard(m.Y, time.Month(m.M))

		return ctx.EditMessageReplyMarkup(gogram.WithEditMessageReplyMarkupReplyMarkup(kb.Markup()))
	})

	c.pick.Handle(rg, func(ctx *gogram.Context, cq *gogram.CallbackQuery, d day) error {
		date := time.Date(d.Y, time.Month(d.M), d.D, 0, 0, 0, 0, c.loc)
		if c.isDisabled(date) {
			return ctx.AnswerCallbackQuery()
		}

		return onPick(ctx, cq, date)
	})

	rg.HandleCallback(c.noop.CallbackData, func(ctx *gogram.Context, _ *gogram.CallbackQuery) error {
		return ctx.AnswerCallbackQuery()
	})
}

func (c *Calendar) isDisabled(d time.Time) bool {
	if !c.min.IsZero() && d.Before(c.min) {
		return true
	}

	if !c.max.IsZero() && d.After(c.max) {
		return true
	}

	return c.disabled != nil && c.disabled(d)
}

func dateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
