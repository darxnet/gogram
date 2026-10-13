package gogram

// Option mutates P. Options are plain closures that the compiler keeps on
// the stack at the call site: applying them costs no allocation.
type Option[P any] func(*P)

// fieldOption returns an option that sets the field selected by field to value.
// The selector is a func rather than an offset so that it inlines away.
func fieldOption[P any, V any](value V, field func(*P) *V) Option[P] {
	return func(params *P) {
		*field(params) = value
	}
}
