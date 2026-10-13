// Package paginator renders a list as pages of inline buttons with a
// navigation row. It is stateless: the page number travels in the callback
// data, so a bot restart or a second instance never loses a user's place.
//
//	users := paginator.New("users",
//		func(ctx *gogram.Context, page, perPage int) ([]User, int, error) {
//			return db.Users(page, perPage)
//		},
//		func(u User) gogram.InlineKeyboardButton { return userCB.Button(u.Name, UserRef{ID: u.ID}) },
//		paginator.PerPage(8), paginator.Columns(2),
//		paginator.Text(func(_ *gogram.Context, page, pages int) string {
//			return fmt.Sprintf("Users, page %d/%d", page, pages)
//		}),
//	)
//	users.Handle(router)
//
//	router.HandleCommand("/users", func(ctx *gogram.Context, _ *gogram.Message) error {
//		return users.Send(ctx, 1)
//	})
package paginator

import (
	"strconv"

	"github.com/darxnet/gogram"
)

// Source loads one page of items (1-based page) and reports the total
// number of items, so the navigation row can show "page/pages".
type Source[T any] func(ctx *gogram.Context, page, perPage int) (items []T, total int, err error)

// nav is the navigation callback data.
type nav struct {
	Page int
}

// Option configures a [Paginator].
type Option func(*config)

type config struct {
	perPage int
	columns int
	labels  [4]string // first, prev, next, last
	counter func(page, pages int) string
	text    func(ctx *gogram.Context, page, pages int) string
	footer  []gogram.InlineKeyboardButton
}

// PerPage sets the number of items per page. Default: 10.
func PerPage(n int) Option { return func(c *config) { c.perPage = n } }

// Columns sets the number of item buttons per row. Default: 1.
func Columns(n int) Option { return func(c *config) { c.columns = n } }

// Labels sets the navigation button labels. Default: « ‹ › ».
func Labels(first, prev, next, last string) Option {
	return func(c *config) { c.labels = [4]string{first, prev, next, last} }
}

// Counter sets the text of the middle navigation button. Default: "page/pages".
func Counter(f func(page, pages int) string) Option { return func(c *config) { c.counter = f } }

// Text sets the message text per page; with it, navigation edits the text
// and the keyboard, without it only the keyboard. [Paginator.Send] needs it.
func Text(f func(ctx *gogram.Context, page, pages int) string) Option {
	return func(c *config) { c.text = f }
}

// Footer adds a last row of buttons, e.g. "Back".
func Footer(buttons ...gogram.InlineKeyboardButton) Option {
	return func(c *config) { c.footer = buttons }
}

// Paginator renders pages of T.
type Paginator[T any] struct {
	cfg    config
	nav    *gogram.Callback[nav]
	source Source[T]
	item   func(T) gogram.InlineKeyboardButton
}

// New creates a paginator. prefix identifies its navigation callbacks and
// must be unique among the router's callbacks; item renders one element
// as a button (its callback data is yours, see [gogram.Callback]).
func New[T any](
	prefix string,
	source Source[T],
	item func(T) gogram.InlineKeyboardButton,
	opts ...Option,
) *Paginator[T] {
	cfg := config{
		perPage: 10,
		columns: 1,
		labels:  [4]string{"«", "‹", "›", "»"},
		counter: func(page, pages int) string { return strconv.Itoa(page) + "/" + strconv.Itoa(pages) },
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Paginator[T]{
		cfg:    cfg,
		nav:    gogram.NewCallback[nav](prefix),
		source: source,
		item:   item,
	}
}

// Keyboard loads page and renders it: item rows, the navigation row and
// the footer. It also returns the clamped page and the number of pages.
func (p *Paginator[T]) Keyboard(
	ctx *gogram.Context,
	page int,
) (kb *gogram.InlineKeyboardBuilder, cur, pages int, err error) {
	page = max(page, 1)

	items, total, err := p.source(ctx, page, p.cfg.perPage)
	if err != nil {
		return nil, 0, 0, err
	}

	pages = max((total+p.cfg.perPage-1)/p.cfg.perPage, 1)

	// Beyond the last page (items deleted meanwhile): load the last one.
	if page > pages {
		page = pages

		if items, _, err = p.source(ctx, page, p.cfg.perPage); err != nil {
			return nil, 0, 0, err
		}
	}

	kb = gogram.NewInlineKeyboardBuilder()

	for i, it := range items {
		if i != 0 && i%p.cfg.columns == 0 {
			kb.Row()
		}

		kb.Add(p.item(it))
	}

	kb.Row()

	if pages > 1 {
		kb.Add(
			p.nav.Button(p.cfg.labels[0], nav{Page: 1}),
			p.nav.Button(p.cfg.labels[1], nav{Page: max(page-1, 1)}),
			p.nav.Button(p.cfg.counter(page, pages), nav{Page: page}),
			p.nav.Button(p.cfg.labels[2], nav{Page: min(page+1, pages)}),
			p.nav.Button(p.cfg.labels[3], nav{Page: pages}),
		).Row()
	}

	kb.Add(p.cfg.footer...).Row()

	return kb, page, pages, nil
}

// Send sends page as a new message. It requires the [Text] option.
func (p *Paginator[T]) Send(ctx *gogram.Context, page int) error {
	kb, page, pages, err := p.Keyboard(ctx, page)
	if err != nil {
		return err
	}

	return ctx.SendMessage(p.text(ctx, page, pages), gogram.WithSendMessageReplyMarkup(kb.ReplyMarkup()))
}

// Handle registers the navigation callback on rg: a press re-renders the
// requested page into the message the button belongs to.
func (p *Paginator[T]) Handle(rg *gogram.RouterGroup) {
	p.nav.Handle(rg, func(ctx *gogram.Context, _ *gogram.CallbackQuery, n nav) error {
		kb, page, pages, err := p.Keyboard(ctx, n.Page)
		if err != nil {
			return err
		}

		if p.cfg.text == nil {
			return ctx.EditMessageReplyMarkup(gogram.WithEditMessageReplyMarkupReplyMarkup(kb.Markup()))
		}

		return ctx.EditMessageText(p.text(ctx, page, pages), gogram.WithEditMessageTextReplyMarkup(kb.Markup()))
	})
}

func (p *Paginator[T]) text(ctx *gogram.Context, page, pages int) string {
	if p.cfg.text == nil {
		return p.cfg.counter(page, pages)
	}

	return p.cfg.text(ctx, page, pages)
}
