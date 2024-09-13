package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Message map[string]any

func NewMessageType(_type string) *Message {
	return NewMessage(map[string]any{"@type": _type})
}

func NewMessageWithType(force_type string, content map[string]any) *Message {
	return NewMessage(content).With("@type", force_type)
}

func NewMessage(content map[string]any) *Message {
	m := make(Message)
	for k, v := range content {
		m[k] = v
	}
	return &m
}

func NewMessageJson(js string) (m *Message, err error) {
	err = json.Unmarshal([]byte(js), m)
	return
}

func NewMessage1(field1 string, value1 any) *Message {
	return NewMessage(map[string]any{field1: value1})
}

func NewMessage2(field1 string, value1 any, field2 string, value2 any) *Message {
	return NewMessage(map[string]any{field1: value1, field2: value2})
}

func NewMessage3(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any) *Message {
	return NewMessage(map[string]any{field1: value1, field2: value2, field3: value3})
}

func NewMessage4(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any, field4 string, value4 any) *Message {
	return NewMessage(map[string]any{field1: value1, field2: value2, field3: value3, field4: value4})
}

func NewMessage5(field1 string, value1 any, field2 string, value2 any, field3 string, value3 any, field4 string, value4 any, field5 string, value5 any) *Message {
	return NewMessage(map[string]any{field1: value1, field2: value2, field3: value3, field4: value4, field5: value5})
}

func (m *Message) With(field string, value any) *Message {
	if value != nil {
		(*m)[field] = value
	}
	return m
}

func (m *Message) SetExtra(extra *string) *Message {
	return m.With("@extra", extra)
}

func (m *Message) Json() (result []byte, err error) {
	result, err = json.Marshal(*m)
	return
}

func (m *Message) Extra() string {
	return m.Fstring("@extra")
}
func (m *Message) Type() string {
	return m.Fstring("@type")
}

func (m *Message) IsType(_t string) bool {
	return m.Type() == _t
}

func (m *Message) IsOk() bool {
	return m.IsType("ok")
}

func (m *Message) ClientId() int {
	return int(m.Fnumber("@client_id"))
}

func (m *Message) Fstring(path string) string {
	return Field[string](m, path)
}

func (m *Message) Fnumber(path string) float64 {
	return Field[float64](m, path)
}

func (m *Message) Fbool(path string) bool {
	return Field[bool](m, path)
}

func (m *Message) Farray(path string) []any {
	return Field[[]any](m, path)
}

func (m *Message) Fobject(path string) *Message {
	k := Field[map[string]any](m, path)
	return NewMessage(k)
}

func (m *Message) Ftimestamp(path string) time.Time {
	return time.Unix(int64(m.Fnumber(path)), 0)
}

func FieldErr[T any](m *Message, path string) (value T, err error) {
	names := strings.Split(path, ".")
	a := *m
	for i, name := range names {
		x, exists := a[name]
		if !exists { //object should exist
			err = fmt.Errorf("key doesn't exist: %v", name)
			return
		}

		if i < len(names)-1 { //is not last key
			y, isObject := x.(map[string]any)
			if !isObject { //and should be object(map)
				err = fmt.Errorf("key value is not a map: %v", name)
				return
			}
			a = y
		} else { //is last key
			z, isNeedType := x.(T)
			if !isNeedType { //and should be of given type
				err = fmt.Errorf("key value is not of required type: %v", name)
				return
			}
			value = z
		}
	}

	return
}

func Field[T any](m *Message, path string) T {
	value, _ := FieldErr[T](m, path)
	// if err != nil {
	// 	slog.Warn("Unable to get value", "error", err)
	// }
	return value
}
