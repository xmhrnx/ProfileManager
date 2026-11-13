package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// object is a JSON object that keeps its keys in file order, so editing
// a few keys of a settings file leaves the rest of it as it was.
type object struct {
	keys []string
	vals map[string]json.RawMessage
}

var bom = []byte{0xEF, 0xBB, 0xBF}

func parseObject(data []byte) (*object, error) {
	data = bytes.TrimPrefix(data, bom)
	o := &object{vals: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return o, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("invalid JSON key")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := o.vals[k]; !dup {
			o.keys = append(o.keys, k)
		}
		o.vals[k] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *object) set(k string, v json.RawMessage) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *object) del(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	for i, x := range o.keys {
		if x == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// marshal writes the object indented with two spaces.
func (o *object) marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n  ")
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteString(": ")
		var v bytes.Buffer
		if err := json.Indent(&v, o.vals[k], "  ", "  "); err != nil {
			return nil, err
		}
		buf.Write(v.Bytes())
	}
	if len(o.keys) > 0 {
		buf.WriteString("\n")
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

// compact normalizes a JSON value's formatting, so a program rewriting
// its file with other whitespace doesn't look like a new sign-in.
func compact(v json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, v); err != nil {
		return v
	}
	return buf.Bytes()
}
