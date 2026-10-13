package gogram

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

// ErrInvalidCallbackData is returned when callback data cannot be encoded
// (too long, a string field with a space) or decoded (wrong prefix, missing
// or malformed fields: typically a button from an older build).
var ErrInvalidCallbackData = errors.New("gogram: invalid callback data")

// callbackField is one exported field of T, read and written through its
// offset so that Encode/Decode never box T into an interface.
type callbackField struct {
	name   string
	offset uintptr
	kind   reflect.Kind
	bits   int // integer width for strconv
}

// Callback encodes a struct T into InlineKeyboardButton.CallbackData as
// "prefix field1 field2 ..." and decodes it back for the handler:
//
//	type Page struct {
//		List string
//		N    int
//	}
//
//	var pageCB = gogram.NewCallback[Page]("pg")
//
//	kb.Add(pageCB.Button("›", Page{List: "users", N: 2})) // callback_data: "pg users 2"
//
//	pageCB.Handle(router, func(ctx *gogram.Context, cq *gogram.CallbackQuery, p Page) error {
//		...
//	})
//
// Fields are encoded positionally, so renaming a field is safe and
// reordering, adding or removing one is not: buttons already shown to users
// then fail to decode with [ErrInvalidCallbackData]. Supported field types:
// bool, integers and strings without spaces. The encoded data must fit
// [CallbackDataMaxLen] bytes.
type Callback[T any] struct {
	prefix string
	fields []callbackField
}

// NewCallback creates a callback-data factory for struct T. It panics when
// prefix is empty or contains a space, or when T is not a struct of
// supported exported fields: both are programming errors.
func NewCallback[T any](prefix string) *Callback[T] {
	if prefix == "" || strings.IndexByte(prefix, ' ') != -1 {
		panic("gogram: callback prefix must be non-empty and contain no spaces")
	}

	var zero T

	typ := reflect.TypeOf(zero)
	if typ == nil || typ.Kind() != reflect.Struct {
		panic("gogram: callback type must be a struct")
	}

	fields := make([]callbackField, 0, typ.NumField())

	for f := range typ.Fields() {
		if !f.IsExported() {
			panic("gogram: callback field " + f.Name + " must be exported")
		}

		field := callbackField{name: f.Name, offset: f.Offset, kind: f.Type.Kind()}

		switch field.kind { //nolint:exhaustive // every other kind is rejected below
		case reflect.Bool, reflect.String:
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.bits = f.Type.Bits()
		default:
			panic("gogram: callback field " + f.Name + " has unsupported type " + f.Type.String())
		}

		fields = append(fields, field)
	}

	return &Callback[T]{prefix: prefix, fields: fields}
}

// Prefix returns the data prefix.
func (cb *Callback[T]) Prefix() string {
	return cb.prefix
}

// Encode returns the callback data for v.
func (cb *Callback[T]) Encode(v T) (string, error) {
	var buf [CallbackDataMaxLen + 1]byte

	b := append(buf[:0], cb.prefix...)
	base := unsafe.Pointer(&v) //nolint:gosec // G103: field offsets come from reflect on T itself

	for i := range cb.fields {
		f := &cb.fields[i]
		p := unsafe.Add(base, f.offset)

		b = append(b, ' ')

		switch f.kind { //nolint:exhaustive // kinds are validated in NewCallback
		case reflect.Bool:
			b = strconv.AppendBool(b, *(*bool)(p))

		case reflect.String:
			s := *(*string)(p)
			if strings.IndexByte(s, ' ') != -1 {
				return "", fmt.Errorf("%w: field %s contains a space", ErrInvalidCallbackData, f.name)
			}

			b = append(b, s...)

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			b = strconv.AppendInt(b, loadInt(p, f.kind), 10)

		default: // unsigned
			b = strconv.AppendUint(b, loadUint(p, f.kind), 10)
		}

		if len(b) > CallbackDataMaxLen {
			return "", fmt.Errorf("%w: %d bytes exceed %d", ErrInvalidCallbackData, len(b), CallbackDataMaxLen)
		}
	}

	return string(b), nil
}

// Decode parses callback data produced by Encode.
func (cb *Callback[T]) Decode(data string) (v T, err error) {
	rest, ok := strings.CutPrefix(data, cb.prefix)
	if !ok {
		return v, fmt.Errorf("%w: prefix %q expected", ErrInvalidCallbackData, cb.prefix)
	}

	base := unsafe.Pointer(&v) //nolint:gosec // G103: field offsets come from reflect on T itself

	for i := range cb.fields {
		f := &cb.fields[i]

		if rest == "" || rest[0] != ' ' {
			return v, fmt.Errorf("%w: field %s missing", ErrInvalidCallbackData, f.name)
		}

		rest = rest[1:]

		token := rest
		if j := strings.IndexByte(rest, ' '); j != -1 {
			token, rest = rest[:j], rest[j:]
		} else {
			rest = ""
		}

		p := unsafe.Add(base, f.offset)

		switch f.kind { //nolint:exhaustive // kinds are validated in NewCallback
		case reflect.Bool:
			*(*bool)(p), err = strconv.ParseBool(token)

		case reflect.String:
			*(*string)(p) = token

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var n int64

			n, err = strconv.ParseInt(token, 10, f.bits)
			storeInt(p, f.kind, n)

		default: // unsigned
			var n uint64

			n, err = strconv.ParseUint(token, 10, f.bits)
			storeUint(p, f.kind, n)
		}

		if err != nil {
			return v, fmt.Errorf("%w: field %s: %w", ErrInvalidCallbackData, f.name, err)
		}
	}

	if rest != "" {
		return v, fmt.Errorf("%w: unexpected trailing data %q", ErrInvalidCallbackData, rest)
	}

	return v, nil
}

// Button returns an inline button with text and the callback data for v.
// It panics when v cannot be encoded: a button is built from program
// constants, so that is a programming error, see Encode to check at runtime.
func (cb *Callback[T]) Button(text string, v T) InlineKeyboardButton {
	data, err := cb.Encode(v)
	if err != nil {
		panic(err)
	}

	return InlineKeyboardButton{Text: text, CallbackData: data}
}

// Match reports whether data was produced by this Callback.
func (cb *Callback[T]) Match(data string) bool {
	return data == cb.prefix ||
		len(data) > len(cb.prefix) && data[len(cb.prefix)] == ' ' && data[:len(cb.prefix)] == cb.prefix
}

// Filter returns a router filter matching callback queries of this Callback.
func (cb *Callback[T]) Filter() Filter {
	return func(ctx *Context) bool {
		cq := ctx.Update().CallbackQuery
		return cq != nil && cb.Match(cq.Data)
	}
}

// Handle registers handler for callback queries of this Callback using the
// router's O(1) prefix lookup. Data that fails to decode is reported to the
// error handler as [ErrInvalidCallbackData].
func (cb *Callback[T]) Handle(rg *RouterGroup, handler func(ctx *Context, cq *CallbackQuery, data T) error) {
	fn := func(ctx *Context) error {
		cq := ctx.Update().CallbackQuery

		v, err := cb.Decode(cq.Data)
		if err != nil {
			return err
		}

		return handler(ctx, cq, v)
	}

	rg.router.handlersCallbacks[cb.prefix] = append(rg.router.handlersCallbacks[cb.prefix], route{
		filter:  rg.filter,
		handler: rg.applyMiddlewares(fn),
	})
}

//nolint:exhaustive // kinds are validated in NewCallback
func loadInt(p unsafe.Pointer, kind reflect.Kind) int64 {
	switch kind {
	case reflect.Int8:
		return int64(*(*int8)(p))
	case reflect.Int16:
		return int64(*(*int16)(p))
	case reflect.Int32:
		return int64(*(*int32)(p))
	case reflect.Int64:
		return *(*int64)(p)
	default:
		return int64(*(*int)(p))
	}
}

// storeInt stores v, parsed with the field's bit size, so the narrowing
// conversions cannot overflow.
//
//nolint:exhaustive,gosec // kinds are validated in NewCallback; G115 see above
func storeInt(p unsafe.Pointer, kind reflect.Kind, v int64) {
	switch kind {
	case reflect.Int8:
		*(*int8)(p) = int8(v)
	case reflect.Int16:
		*(*int16)(p) = int16(v)
	case reflect.Int32:
		*(*int32)(p) = int32(v)
	case reflect.Int64:
		*(*int64)(p) = v
	default:
		*(*int)(p) = int(v)
	}
}

//nolint:exhaustive // kinds are validated in NewCallback
func loadUint(p unsafe.Pointer, kind reflect.Kind) uint64 {
	switch kind {
	case reflect.Uint8:
		return uint64(*(*uint8)(p))
	case reflect.Uint16:
		return uint64(*(*uint16)(p))
	case reflect.Uint32:
		return uint64(*(*uint32)(p))
	case reflect.Uint64:
		return *(*uint64)(p)
	default:
		return uint64(*(*uint)(p))
	}
}

// storeUint stores v, parsed with the field's bit size, so the narrowing
// conversions cannot overflow.
//
//nolint:exhaustive,gosec // kinds are validated in NewCallback; G115 see above
func storeUint(p unsafe.Pointer, kind reflect.Kind, v uint64) {
	switch kind {
	case reflect.Uint8:
		*(*uint8)(p) = uint8(v)
	case reflect.Uint16:
		*(*uint16)(p) = uint16(v)
	case reflect.Uint32:
		*(*uint32)(p) = uint32(v)
	case reflect.Uint64:
		*(*uint64)(p) = v
	default:
		*(*uint)(p) = uint(v)
	}
}
