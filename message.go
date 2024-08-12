package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Message map[string]interface{}

func NewMessage(_type string, content map[string]interface{}) *Message {
	m := make(Message)

	for k, v := range content {
		m[k] = v
	}

	if len(_type) > 0 {
		m["@type"] = _type
	}

	return &m
}

func (m *Message) SetExtra(extra *string) {
	if extra != nil {
		(*m)["@extra"] = extra
	}
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
	return field[string](m, path)
}

func (m *Message) Fnumber(path string) float64 {
	return field[float64](m, path)
}

func (m *Message) Fbool(path string) bool {
	return field[bool](m, path)
}

func (m *Message) Farray(path string) []interface{} {
	return field[[]interface{}](m, path)
}

/*
 */
func fieldErr[T any](m *Message, path string) (value T, err error) {
	names := strings.Split(path, ".")
	a := *m
	for i, name := range names {
		x, exists := a[name]
		if !exists { //object should exist
			err = fmt.Errorf("key doesn't exist: %v", name)
			return
		}

		if i < len(names)-1 { //is not last key
			y, isObject := x.(map[string]interface{})
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

func field[T any](m *Message, path string) T {
	value, _ := fieldErr[T](m, path)
	// if err != nil {
	// 	slog.Warn("Unable to get value", "error", err)
	// }
	return value
}
