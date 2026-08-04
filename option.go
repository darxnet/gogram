package gogram

// Option mutates P and returns an option that restores its previous state.
type Option[P any] func(*P) Option[P]

func fieldOption[P any, V any](value V, field func(*P) *V) Option[P] {
	return func(params *P) Option[P] {
		target := field(params)
		previous := *target
		*target = value

		return fieldOption(previous, field)
	}
}
