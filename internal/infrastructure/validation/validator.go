package validation

import (
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// Get returns the singleton validator instance
func Get() *validator.Validate {
	once.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())

		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			if name != "" {
				return name
			}
			return fld.Name
		})

		_ = validate.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
			if fl.Field().Kind() != reflect.String {
				return false
			}
			return strings.TrimSpace(fl.Field().String()) != ""
		})

		_ = validate.RegisterValidation("http_url", func(fl validator.FieldLevel) bool {
			if fl.Field().Kind() != reflect.String {
				return false
			}
			raw := strings.TrimSpace(fl.Field().String())
			if raw == "" {
				return false
			}
			u, err := url.Parse(raw)
			if err != nil {
				return false
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return false
			}
			if strings.TrimSpace(u.Host) == "" {
				return false
			}
			return true
		})

		_ = validate.RegisterValidation("future", func(fl validator.FieldLevel) bool {
			field := fl.Field()
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					return true
				}
				field = field.Elem()
			}
			if field.Type() != reflect.TypeOf(time.Time{}) {
				return false
			}
			t := field.Interface().(time.Time)
			return t.After(time.Now())
		})
	})
	return validate
}

// Validate validates a struct and returns an error if invalid
func Validate(s any) error {
	return Get().Struct(s)
}
