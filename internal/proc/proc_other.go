//go:build !linux && !darwin && !windows

package proc

import "errors"

type system struct{}

func (system) List() ([]Process, error) { return nil, errors.New("proc: not supported on this system") }
func (system) Quit(int) error           { return errors.New("proc: not supported on this system") }
