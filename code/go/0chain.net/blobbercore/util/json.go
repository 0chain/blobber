package util

import (
	"fmt"
	"reflect"
	"strings"
)

// Validate unmarshalled data with tag-based rules
// Example:
// struct {
//	Name string `json:"name" validation:"required"`
// }
func UnmarshalValidation(v interface{}) error {
	fields := reflect.ValueOf(v).Elem()

	for i := 0; i < fields.NumField(); i++ {
		validation := fields.Type().Field(i).Tag.Get("validation")
		if strings.Contains(validation, "required") && fields.Field(i).IsZero() {
			// todo: better try this first:
			// jsonFieldName := fields.Type().Field(i).Tag.Get("json")
			return fmt.Errorf("The '%s' field is required", fields.Type().Field(i).Name)
		}
	}

	return nil
}
