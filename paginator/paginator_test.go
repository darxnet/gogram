package paginator_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/darxnet/gogram"
	"github.com/darxnet/gogram/paginator"
	"github.com/darxnet/gogram/tgtest"
)

func TestPaginator(t *testing.T) {
	t.Parallel()

	all := make([]int, 23)
	for i := range all {
		all[i] = i + 1
	}

	source := func(_ *gogram.Context, page, perPage int) ([]int, int, error) {
		from := min((page-1)*perPage, len(all))
		to := min(from+perPage, len(all))

		return all[from:to], len(all), nil
	}

	p := paginator.New("nums", source,
		func(n int) gogram.InlineKeyboardButton {
			return gogram.InlineKeyboardButton{Text: strconv.Itoa(n), CallbackData: "x"}
		},
		paginator.PerPage(10), paginator.Columns(5),
		paginator.Text(func(_ *gogram.Context, page, pages int) string { return fmt.Sprintf("Page %d of %d", page, pages) }),
		paginator.Footer(gogram.InlineKeyboardButton{Text: "Back", CallbackData: "back"}),
	)

	rec := tgtest.NewRecorder()
	router := gogram.NewRouter()
	p.Handle(router.RouterGroup)

	client, err := gogram.NewClient("1:a", gogram.WithCaller(rec), gogram.WithRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	ctx := tgtest.NewContext(t, client, tgtest.Text(7, "/nums"))

	kb, page, pages, err := p.Keyboard(ctx, 1)
	if err != nil || page != 1 || pages != 3 {
		t.Fatalf("Keyboard = page %d of %d, %v", page, pages, err)
	}

	rows := kb.Rows()
	// 2 item rows of 5, navigation row, footer row.
	if len(rows) != 4 || len(rows[0]) != 5 || len(rows[1]) != 5 || len(rows[2]) != 5 || rows[3][0].Text != "Back" {
		t.Fatalf("rows = %+v", rows)
	}

	if rows[2][2].Text != "1/3" || rows[2][3].CallbackData != "nums 2" || rows[2][4].CallbackData != "nums 3" {
		t.Fatalf("nav row = %+v", rows[2])
	}

	// Last page is short and clamps requests beyond the end.
	kb, page, _, err = p.Keyboard(ctx, 99)
	if err != nil || page != 3 || len(kb.Rows()[0]) != 3 {
		t.Fatalf("last page = %d, rows %+v, %v", page, kb.Rows(), err)
	}

	if err = p.Send(ctx, 1); err != nil {
		t.Fatal(err)
	}

	rec.AssertCalled(t, "sendMessage", tgtest.Match(func(sp *gogram.SendMessageParams) bool {
		return sp.Text == "Page 1 of 3" && sp.ReplyMarkup != nil && len(sp.ReplyMarkup.InlineKeyboard) == 4
	}))

	// Pressing "›" edits the message to page 2.
	router.Process(tgtest.NewContext(t, client, tgtest.Callback(7, "nums 2")))

	rec.AssertCalled(t, "editMessageText", tgtest.Match(func(ep *gogram.EditMessageTextParams) bool {
		return ep.Text == "Page 2 of 3" && ep.ReplyMarkup.InlineKeyboard[2][2].Text == "2/3"
	}))
}
