//go:build !linux && !freebsd && !openbsd && !netbsd && !darwin && !windows

package keyring

type system struct{}

func (system) Get(Item) ([]byte, error) { return nil, ErrUnsupported }
func (system) Set(Item, []byte) error   { return ErrUnsupported }
func (system) Delete(Item) error        { return ErrUnsupported }
