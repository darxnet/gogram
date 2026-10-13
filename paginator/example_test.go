package paginator_test

import (
	"fmt"
	"os"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/paginator"
)

type user struct {
	ID   int64
	Name string
}

func Example() {
	users := []user{{1, "Ann"}, {2, "Bob"}, {3, "Cid"}}

	userCB := gogram.NewCallback[struct{ ID int64 }]("user")

	p := paginator.New("users",
		func(_ *gogram.Context, page, perPage int) ([]user, int, error) {
			from := min((page-1)*perPage, len(users))
			return users[from:min(from+perPage, len(users))], len(users), nil
		},
		func(u user) gogram.InlineKeyboardButton { return userCB.Button(u.Name, struct{ ID int64 }{u.ID}) },
		paginator.PerPage(2),
		paginator.Text(func(_ *gogram.Context, page, pages int) string { return fmt.Sprintf("Users %d/%d", page, pages) }),
	)

	router := gogram.NewRouter()
	p.Handle(router.RouterGroup)

	router.HandleCommand("/users", func(ctx *gogram.Context, _ *gogram.Message) error {
		return p.Send(ctx, 1)
	})

	userCB.Handle(router.RouterGroup, func(ctx *gogram.Context, _ *gogram.CallbackQuery, d struct{ ID int64 }) error {
		return ctx.AnswerCallbackQuery(gogram.WithAnswerCallbackQueryText(fmt.Sprintf("user %d", d.ID)))
	})

	_, _ = gogram.NewClient(os.Getenv("BOT_TOKEN"), gogram.WithRouter(router))

	fmt.Println(userCB.Button("Ann", struct{ ID int64 }{1}).CallbackData)
	// Output: user 1
}
