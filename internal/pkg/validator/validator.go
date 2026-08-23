package validator

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates a struct using `validate` tags and returns a map of
// field name -> human readable error message.
func ValidateStruct(s interface{}) map[string]string {
	errs := make(map[string]string)
	err := validate.Struct(s)
	if err == nil {
		return errs
	}
	// Use json tag names when available for friendlier keys.
	var st reflect.Type
	v := reflect.ValueOf(s)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	st = v.Type()

	if verrs, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range verrs {
			field := fe.Field()
			if f, ok := st.FieldByName(field); ok {
				if name := f.Tag.Get("json"); name != "" && name != "-" {
					field = strings.Split(name, ",")[0]
				}
			}
			errs[field] = messageForTag(fe)
		}
	}
	return errs
}

// Var validates a single variable.
func Var(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

func messageForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Must be a valid email address"
	case "min":
		return "Value is too short"
	case "max":
		return "Value is too long"
	case "len":
		return "Value has incorrect length"
	case "gte":
		return "Value is too small"
	case "lte":
		return "Value is too large"
	default:
		return "Invalid value"
	}
}
