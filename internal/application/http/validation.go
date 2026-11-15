package http

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/zeann3th/policy-service/internal/core"
	"golang.org/x/text/currency"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("iso4217", func(fl validator.FieldLevel) bool {
			_, err := currency.ParseISO(fl.Field().String())
			return err == nil
		})
	}
}

func badRequest(err error, value any) error {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return core.ErrBadRequest.WithMessage(err.Error()).WithCause(err)
	}
	t := reflect.TypeOf(value)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	messages := make([]string, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		field, _ := t.FieldByName(fieldErr.StructField())
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = fieldErr.Field()
		}
		switch fieldErr.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", name))
		case "iso4217":
			messages = append(messages, fmt.Sprintf("%s must be a valid ISO 4217 currency", name))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", name))
		}
	}
	return core.ErrBadRequest.WithMessage(strings.Join(messages, ", ")).WithCause(err)
}
