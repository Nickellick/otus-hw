package hw09structvalidator

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrNotStruct      = errors.New("input must be a struct")
	ErrInvalidRule    = errors.New("invalid validation rule")
	ErrUnsupported    = errors.New("unsupported field type")
	ErrInvalidLength  = errors.New("invalid string length")
	ErrNotMatching    = errors.New("regexp mismatch")
	ErrNotInSet       = errors.New("value not in allowed set")
	ErrLessThanMin    = errors.New("value less than minimum")
	ErrGreaterThanMax = errors.New("value greater than maximum")
)

const (
	minToken    = "min"
	maxToken    = "max"
	inToken     = "in"
	lenToken    = "len"
	regexpToken = "regexp"
)

type ValidationError struct {
	Field string
	Err   error
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	messages := make([]string, 0, len(v))

	for _, validationErr := range v {
		messages = append(
			messages,
			fmt.Sprintf("%s: %v", validationErr.Field, validationErr.Err),
		)
	}

	return strings.Join(messages, "; ")
}

func Validate(v interface{}) error {
	value := reflect.ValueOf(v)

	if !value.IsValid() || value.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	valueType := value.Type()
	validationErrors := make(ValidationErrors, 0)

	for i := 0; i < value.NumField(); i++ {
		fieldType := valueType.Field(i)

		if !fieldType.IsExported() {
			continue
		}

		tag, exists := fieldType.Tag.Lookup("validate")
		if !exists {
			continue
		}

		fieldValue := value.Field(i)

		for _, rule := range strings.Split(tag, "|") {
			errs, err := validateField(fieldValue, rule)
			if err != nil {
				return fmt.Errorf("field %s: %w", fieldType.Name, err)
			}

			for _, validationErr := range errs {
				validationErrors = append(validationErrors, ValidationError{
					Field: fieldType.Name,
					Err:   validationErr,
				})
			}
		}
	}

	if len(validationErrors) > 0 {
		return validationErrors
	}

	return nil
}

func validateField(value reflect.Value, rule string) ([]error, error) {
	//nolint:exhaustive // Other reflect.Kind values are handled by "default" case
	switch value.Kind() {
	case reflect.Int:
		return validationResult(validateIntField(value, rule))

	case reflect.String:
		return validationResult(validateStringField(value, rule))

	case reflect.Slice:
		var validationErrors []error

		for i := 0; i < value.Len(); i++ {
			errs, err := validateField(value.Index(i), rule)
			if err != nil {
				return nil, err
			}

			validationErrors = append(validationErrors, errs...)
		}

		return validationErrors, nil

	default:
		return nil, ErrUnsupported
	}
}

func validateIntField(value reflect.Value, rule string) error {
	name, ruleValue, ok := strings.Cut(rule, ":")
	if !ok {
		return ErrInvalidRule
	}

	switch name {
	case minToken:
		minValue, err := strconv.Atoi(ruleValue)
		if err != nil {
			return fmt.Errorf("%w: invalid min value: %v", ErrInvalidRule, ruleValue)
		}

		if value.Int() < int64(minValue) {
			return ErrLessThanMin
		}

	case maxToken:
		maxValue, err := strconv.Atoi(ruleValue)
		if err != nil {
			return fmt.Errorf("%w: invalid max value: %v", ErrInvalidRule, ruleValue)
		}

		if value.Int() > int64(maxValue) {
			return ErrGreaterThanMax
		}
	case inToken:
		allowedValues := strings.Split(ruleValue, ",")
		found := false

		for _, item := range allowedValues {
			allowed, err := strconv.Atoi(item)
			if err != nil {
				return fmt.Errorf("%w: invalid in value: %v", ErrInvalidRule, item)
			}
			if value.Int() == int64(allowed) {
				found = true
			}
		}
		if !found {
			return ErrNotInSet
		}

	default:
		return fmt.Errorf("%w: unknown rule %q", ErrInvalidRule, name)
	}

	return nil
}

func validateStringField(value reflect.Value, rule string) error {
	name, ruleValue, ok := strings.Cut(rule, ":")
	if !ok {
		return ErrInvalidRule
	}

	switch name {
	case lenToken:
		lenValue, err := strconv.Atoi(ruleValue)
		if err != nil || lenValue < 0 {
			return fmt.Errorf("%w: invalid string length: %v", ErrInvalidRule, ruleValue)
		}

		if utf8.RuneCountInString(value.String()) != lenValue {
			return ErrInvalidLength
		}
	case regexpToken:
		regexpValue, err := regexp.Compile(ruleValue)
		if err != nil {
			return fmt.Errorf("%w: invalid string regexp: %v", ErrInvalidRule, ruleValue)
		}

		if !regexpValue.MatchString(value.String()) {
			return ErrNotMatching
		}
	case inToken:
		allowedValues := strings.Split(ruleValue, ",")
		found := false

		for _, item := range allowedValues {
			if value.String() == item {
				found = true
			}
		}
		if !found {
			return ErrNotInSet
		}

	default:
		return fmt.Errorf("%w: unknown rule %q", ErrInvalidRule, name)
	}
	return nil
}

func validationResult(err error) ([]error, error) {
	if err == nil {
		return nil, nil
	}

	if errors.Is(err, ErrInvalidRule) ||
		errors.Is(err, ErrUnsupported) {
		return nil, err
	}

	return []error{err}, nil
}
