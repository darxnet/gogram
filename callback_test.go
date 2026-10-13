package gogram_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

type pageData struct {
	List string
	N    int
	Back bool
	Tag  uint8
}

var pageCB = gogram.NewCallback[pageData]("pg")

func TestCallback_RoundTrip(t *testing.T) {
	t.Parallel()

	in := pageData{List: "users", N: -3, Back: true, Tag: 200}

	data, err := pageCB.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if data != "pg users -3 true 200" {
		t.Fatalf("data = %q", data)
	}

	out, err := pageCB.Decode(data)
	if err != nil || out != in {
		t.Fatalf("Decode = %+v, %v; want %+v", out, err, in)
	}

	btn := pageCB.Button("›", in)
	if btn.Text != "›" || btn.CallbackData != data {
		t.Fatalf("Button = %+v", btn)
	}

	if !pageCB.Match(data) || !pageCB.Match("pg") || pageCB.Match("pgx 1") || pageCB.Match("p") {
		t.Fatal("Match mismatch")
	}
}

func TestCallback_Errors(t *testing.T) {
	t.Parallel()

	if _, err := pageCB.Encode(pageData{List: "has space"}); !errors.Is(err, gogram.ErrInvalidCallbackData) {
		t.Fatalf("space in string: %v", err)
	}

	if _, err := pageCB.Encode(pageData{List: strings.Repeat("x", 70)}); !errors.Is(err, gogram.ErrInvalidCallbackData) {
		t.Fatalf("too long: %v", err)
	}

	invalid := []string{
		"", "xx users 1 true 1", "pg users", "pg users x true 1", "pg users 1 true 300", "pg users 1 true 1 extra",
	}

	for _, data := range invalid {
		if _, err := pageCB.Decode(data); !errors.Is(err, gogram.ErrInvalidCallbackData) {
			t.Fatalf("Decode(%q) = %v, want ErrInvalidCallbackData", data, err)
		}
	}
}

type emptyData struct{}

func TestCallback_NoFields(t *testing.T) {
	t.Parallel()

	cb := gogram.NewCallback[emptyData]("close")

	if cb.Button("x", emptyData{}).CallbackData != "close" {
		t.Fatal("zero-field data must be the bare prefix")
	}

	if _, err := cb.Decode("close"); err != nil {
		t.Fatal(err)
	}
}

func TestCallback_Handle(t *testing.T) {
	t.Parallel()

	r := gogram.NewRouter()

	var (
		got     pageData
		handled int
		errs    []error
	)

	r.SetHandlerErr(func(_ *gogram.Context, err error) { errs = append(errs, err) })

	pageCB.Handle(r.RouterGroup, func(_ *gogram.Context, _ *gogram.CallbackQuery, p pageData) error {
		got = p
		handled++
		return nil
	})

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	process := func(data string) {
		update := &gogram.Update{CallbackQuery: &gogram.CallbackQuery{Data: data}}
		r.Process(gogram.NewTestContext(t.Context(), client, update))
	}

	process("pg users 2 false 1")

	if handled != 1 || got != (pageData{List: "users", N: 2, Tag: 1}) {
		t.Fatalf("handled=%d got=%+v", handled, got)
	}

	process("pg users") // button from an older build

	if handled != 1 || len(errs) != 1 || !errors.Is(errs[0], gogram.ErrInvalidCallbackData) {
		t.Fatalf("handled=%d errs=%v", handled, errs)
	}

	process("other 1")

	if handled != 1 {
		t.Fatal("foreign prefix must not match")
	}
}

func BenchmarkCallback_Encode(b *testing.B) {
	in := pageData{List: "users", N: 12345, Back: true, Tag: 7}

	b.ReportAllocs()

	for range b.N {
		if _, err := pageCB.Encode(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallback_Decode(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		if _, err := pageCB.Decode("pg users 12345 true 7"); err != nil {
			b.Fatal(err)
		}
	}
}
