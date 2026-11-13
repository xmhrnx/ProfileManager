//go:build linux || freebsd || openbsd || netbsd

package keyring

import (
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

// The freedesktop Secret Service API:
// https://specifications.freedesktop.org/secret-service/
const (
	ssName       = "org.freedesktop.secrets"
	ssPath       = dbus.ObjectPath("/org/freedesktop/secrets")
	ssDefault    = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
	ssService    = "org.freedesktop.Secret.Service"
	ssCollection = "org.freedesktop.Secret.Collection"
	ssItem       = "org.freedesktop.Secret.Item"
	ssPrompt     = "org.freedesktop.Secret.Prompt"
	noPrompt     = dbus.ObjectPath("/")
)

type secret struct {
	Session     dbus.ObjectPath
	Params      []byte
	Value       []byte
	ContentType string
}

type system struct{}

type conn struct {
	bus     *dbus.Conn
	session dbus.ObjectPath
}

func open() (*conn, error) {
	bus, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	var out dbus.Variant
	var session dbus.ObjectPath
	// "plain": the session bus is reachable by this user's processes only.
	err = bus.Object(ssName, ssPath).Call(ssService+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&out, &session)
	if err != nil {
		return nil, fmt.Errorf("%w: no Secret Service running (%v)", ErrUnsupported, err)
	}
	return &conn{bus: bus, session: session}, nil
}

func (c *conn) close() {
	c.bus.Object(ssName, c.session).Call("org.freedesktop.Secret.Session.Close", 0)
}

func attrs(it Item) map[string]string {
	if len(it.Attributes) > 0 {
		return it.Attributes
	}
	// The attributes go-keyring and libsecret's generic schema use.
	return map[string]string{"service": it.Service, "username": it.Account}
}

// prompt runs a Secret Service prompt (unlocking the keyring) and
// waits for the user, for up to two minutes.
func (c *conn) prompt(p dbus.ObjectPath) error {
	if p == noPrompt || p == "" {
		return nil
	}
	rule := fmt.Sprintf("type='signal',interface='%s',member='Completed',path='%s'", ssPrompt, p)
	if err := c.bus.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
		return err
	}
	defer c.bus.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule)
	ch := make(chan *dbus.Signal, 4)
	c.bus.Signal(ch)
	defer c.bus.RemoveSignal(ch)
	if err := c.bus.Object(ssName, p).Call(ssPrompt+".Prompt", 0, "").Err; err != nil {
		return err
	}
	timeout := time.After(2 * time.Minute)
	for {
		select {
		case s := <-ch:
			if s.Path != p || len(s.Body) == 0 {
				continue
			}
			if dismissed, _ := s.Body[0].(bool); dismissed {
				return errors.New("keyring: unlock was dismissed")
			}
			return nil
		case <-timeout:
			return errors.New("keyring: timed out waiting for the keyring to unlock")
		}
	}
}

func (c *conn) unlock(paths []dbus.ObjectPath) error {
	var unlocked []dbus.ObjectPath
	var p dbus.ObjectPath
	if err := c.bus.Object(ssName, ssPath).Call(ssService+".Unlock", 0, paths).Store(&unlocked, &p); err != nil {
		return err
	}
	return c.prompt(p)
}

func (c *conn) find(it Item) (dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	err := c.bus.Object(ssName, ssPath).Call(ssService+".SearchItems", 0, attrs(it)).Store(&unlocked, &locked)
	if err != nil {
		return "", err
	}
	if len(unlocked) > 0 {
		return unlocked[0], nil
	}
	if len(locked) > 0 {
		if err := c.unlock(locked[:1]); err != nil {
			return "", err
		}
		return locked[0], nil
	}
	return "", ErrNotFound
}

func (system) Get(it Item) ([]byte, error) {
	c, err := open()
	if err != nil {
		return nil, err
	}
	defer c.close()
	path, err := c.find(it)
	if err != nil {
		return nil, err
	}
	var s secret
	if err := c.bus.Object(ssName, path).Call(ssItem+".GetSecret", 0, c.session).Store(&s); err != nil {
		return nil, err
	}
	return s.Value, nil
}

func (system) Set(it Item, value []byte) error {
	c, err := open()
	if err != nil {
		return err
	}
	defer c.close()
	if err := c.unlock([]dbus.ObjectPath{ssDefault}); err != nil {
		return err
	}
	label := it.Label
	if label == "" {
		label = it.Service
		if it.Account != "" {
			label += " (" + it.Account + ")"
		}
	}
	props := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label":      dbus.MakeVariant(label),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(attrs(it)),
	}
	s := secret{Session: c.session, Value: value, ContentType: "text/plain"}
	var item, p dbus.ObjectPath
	if err := c.bus.Object(ssName, ssDefault).Call(ssCollection+".CreateItem", 0, props, s, true).Store(&item, &p); err != nil {
		return err
	}
	return c.prompt(p)
}

func (system) Delete(it Item) error {
	c, err := open()
	if err != nil {
		return err
	}
	defer c.close()
	path, err := c.find(it)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var p dbus.ObjectPath
	if err := c.bus.Object(ssName, path).Call(ssItem+".Delete", 0).Store(&p); err != nil {
		return err
	}
	return c.prompt(p)
}
