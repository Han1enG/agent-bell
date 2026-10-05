//go:build !cgo

package attention

import "errors"

type Store struct{}
type Transition struct{ ID, Type, Timestamp string }

func OpenStore(string, bool) (*Store, error) {
	return nil, errors.New("SQLite requires a CGO-enabled release build")
}
func (*Store) Close()                           {}
func (*Store) Load(*Engine) error               { return errors.New("SQLite unavailable") }
func (*Store) Save(*Engine, []Transition) error { return errors.New("SQLite unavailable") }
func (*Store) Health() error                    { return errors.New("SQLite unavailable") }
