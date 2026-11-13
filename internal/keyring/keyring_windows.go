//go:build windows

package keyring

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Credential Manager through advapi32's Cred* functions.
var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type system struct{}

func (system) Get(it Item) ([]byte, error) {
	target, err := windows.UTF16PtrFromString(it.Service)
	if err != nil {
		return nil, err
	}
	var pcred *credential
	r, _, err := procCredRead.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&pcred)))
	if r == 0 {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))
	blob := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	return append([]byte(nil), blob...), nil
}

func (system) Set(it Item, value []byte) error {
	target, err := windows.UTF16PtrFromString(it.Service)
	if err != nil {
		return err
	}
	user, err := windows.UTF16PtrFromString(it.Account)
	if err != nil {
		return err
	}
	c := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		UserName:           user,
		Persist:            credPersistLocalMachine,
		CredentialBlobSize: uint32(len(value)),
	}
	if len(value) > 0 {
		c.CredentialBlob = &value[0]
	}
	r, _, err := procCredWrite.Call(uintptr(unsafe.Pointer(&c)), 0)
	if r == 0 {
		return err
	}
	return nil
}

func (system) Delete(it Item) error {
	target, err := windows.UTF16PtrFromString(it.Service)
	if err != nil {
		return err
	}
	r, _, err := procCredDelete.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if r == 0 && !errors.Is(err, windows.ERROR_NOT_FOUND) {
		return err
	}
	return nil
}
