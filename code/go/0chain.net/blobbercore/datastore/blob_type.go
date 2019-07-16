package datastore

import (
	"database/sql/driver"
	"errors"
)

type JSONString []byte

func (data *JSONString) Scan(value interface{}) error {
	if b, ok := value.(string); ok {
		*data = []byte(b)
		return nil
	}
	return errors.New("String expected")
}

func (data JSONString) Value() (driver.Value, error) {
	return string(data), nil
}
