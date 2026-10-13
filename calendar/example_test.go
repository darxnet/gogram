package calendar_test

import (
	"fmt"
	"os"
	"time"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/calendar"
)

func Example() {
	cal := calendar.New("cal",
		calendar.WeekStart(time.Monday),
		calendar.Min(time.Now()),
		calendar.Disabled(func(d time.Time) bool { return d.Weekday() == time.Sunday }),
	)

	router := gogram.NewRouter()

	router.HandleCommand("/book", func(ctx *gogram.Context, _ *gogram.Message) error {
		now := time.Now()
		kb := cal.Keyboard(now.Year(), now.Month())

		return ctx.SendMessage("Pick a date", gogram.WithSendMessageReplyMarkup(kb.ReplyMarkup()))
	})

	cal.Handle(router.RouterGroup, func(ctx *gogram.Context, _ *gogram.CallbackQuery, date time.Time) error {
		return ctx.EditMessageText("Booked " + date.Format("2006-01-02"))
	})

	_, _ = gogram.NewClient(os.Getenv("BOT_TOKEN"), gogram.WithRouter(router))

	fmt.Println(len(cal.Keyboard(2026, time.October).Rows()))
	// Output: 7
}
