package calendar_test

import (
	"testing"
	"time"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/calendar"
	"github.com/darxnet/gogram/tgtest"
)

func TestCalendar_Keyboard(t *testing.T) {
	t.Parallel()

	// October 2026 starts on a Thursday and has 31 days.
	cal := calendar.New("cal",
		calendar.Min(time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)),
		calendar.Disabled(func(d time.Time) bool { return d.Weekday() == time.Sunday }),
	)

	rows := cal.Keyboard(2026, time.October).Rows()

	// header, weekdays, 5 weeks.
	if len(rows) != 7 {
		t.Fatalf("rows = %d", len(rows))
	}

	if rows[0][1].Text != "October 2026" || rows[0][0].CallbackData != "calm 2026 9" ||
		rows[0][2].CallbackData != "calm 2026 11" {
		t.Fatalf("header = %+v", rows[0])
	}

	if rows[1][0].Text != "Mo" || rows[1][6].Text != "Su" {
		t.Fatalf("weekdays = %+v", rows[1])
	}

	week1 := rows[2]
	// Mon–Wed blank, Thu 1 .. Sun 4: all before Min, so blank too.
	for i, b := range week1 {
		if b.Text != " " {
			t.Fatalf("week1[%d] = %+v, want blank", i, b)
		}
	}

	week2 := rows[3]
	if week2[0].CallbackData != "cald 2026 10 5" || week2[6].Text != " " { // Mon 5 enabled, Sun 11 disabled
		t.Fatalf("week2 = %+v", week2)
	}

	last := rows[6]
	if last[5].Text != "31" || last[6].Text != " " { // Sat 31, then trailing blank
		t.Fatalf("last week = %+v", last)
	}

	for i, row := range rows[1:] {
		if len(row) != 7 {
			t.Fatalf("row %d width %d: %+v", i+1, len(row), row)
		}
	}
}

func TestCalendar_Handle(t *testing.T) {
	t.Parallel()

	cal := calendar.New("cal", calendar.Max(time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)))

	rec := tgtest.NewRecorder()
	router := gogram.NewRouter()

	var picked time.Time

	cal.Handle(router.RouterGroup, func(_ *gogram.Context, _ *gogram.CallbackQuery, date time.Time) error {
		picked = date
		return nil
	})

	client, err := gogram.NewClient("1:a", gogram.WithCaller(rec), gogram.WithRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	press := func(data string) { router.Process(tgtest.NewContext(t, client, tgtest.Callback(7, data))) }

	press("calm 2026 11")
	rec.AssertCalled(t, "editMessageReplyMarkup", tgtest.Match(func(p *gogram.EditMessageReplyMarkupParams) bool {
		return p.ReplyMarkup.InlineKeyboard[0][1].Text == "November 2026"
	}))

	press("cald 2026 10 12")

	if !picked.Equal(time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("picked = %v", picked)
	}

	press("cald 2026 10 25") // after Max: answered, not picked
	press("calx")            // blank cell

	if picked.Day() != 12 {
		t.Fatalf("disabled day was picked: %v", picked)
	}

	rec.AssertSequence(t, "editMessageReplyMarkup", "answerCallbackQuery", "answerCallbackQuery")
}
