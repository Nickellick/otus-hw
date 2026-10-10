package hw09structvalidator

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type UserRole string

// Test the function on different structures and other types.
type (
	User struct {
		ID     string `json:"id" validate:"len:36"`
		Name   string
		Age    int             `validate:"min:18|max:50"`
		Email  string          `validate:"regexp:^\\w+@\\w+\\.\\w+$"`
		Role   UserRole        `validate:"in:admin,stuff"`
		Phones []string        `validate:"len:11"`
		meta   json.RawMessage //nolint:unused
	}

	App struct {
		Version string `validate:"len:5"`
	}

	Token struct {
		Header    []byte
		Payload   []byte
		Signature []byte
	}

	Response struct {
		Code int    `validate:"in:200,404,500"`
		Body string `json:"omitempty"`
	}
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		in          interface{}
		expectedErr error
	}{
		{
			name: "valid user",
			in: User{
				ID:     "12345678-1234-1234-1234-123456789012",
				Age:    25,
				Email:  "user@example.com",
				Role:   "admin",
				Phones: []string{"79991234567"},
			},
		},
		{
			name: "invalid age min",
			in: User{
				Age: 10,
			},
			expectedErr: ErrLessThanMin,
		},
		{
			name: "invalid age max",
			in: User{
				Age: 60,
			},
			expectedErr: ErrGreaterThanMax,
		},
		{
			name: "invalid app version",
			in: App{
				Version: "1.0",
			},
			expectedErr: ErrInvalidLength,
		},
		{
			name: "valid response",
			in: Response{
				Code: 200,
			},
		},
		{
			name: "invalid response code",
			in: Response{
				Code: 403,
			},
			expectedErr: ErrNotInSet,
		},
		{
			name: "missing regexp match",
			in: struct {
				Email string `validate:"regexp:^\\d+$"`
			}{
				Email: "hello",
			},
			expectedErr: ErrNotMatching,
		},
		{
			name: "invalid rule",
			in: struct {
				Age int `validate:"banana:123"`
			}{
				Age: 20,
			},
			expectedErr: ErrInvalidRule,
		},
		{
			name: "invalid regexp",
			in: struct {
				Name string `validate:"regexp:["`
			}{
				Name: "hello",
			},
			expectedErr: ErrInvalidRule,
		},
		{
			name:        "not a struct",
			in:          42,
			expectedErr: ErrNotStruct,
		},
		{
			name: "struct without validation tags",
			in: Token{
				Header: []byte{1, 2, 3},
			},
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			tt := tt
			t.Parallel()
			err := Validate(tt.in)

			if tt.expectedErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.expectedErr)
			}

			if errors.Is(err, tt.expectedErr) {
				return
			}

			var validationErrors ValidationErrors
			if !errors.As(err, &validationErrors) {
				t.Fatalf(
					"expected ValidationErrors, got %T: %v",
					err,
					err,
				)
			}

			for _, validationErr := range validationErrors {
				if errors.Is(validationErr.Err, tt.expectedErr) {
					return
				}
			}

			t.Errorf(
				"expected error %v, got %v",
				tt.expectedErr,
				err,
			)
			_ = tt
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	value := struct {
		Age    int      `validate:"min:18|max:50"`
		Phones []string `validate:"len:11"`
	}{
		Age: 10,
		Phones: []string{
			"123",
			"456",
			"79991234567",
		},
	}

	err := Validate(value)

	var validationErrors ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}

	if len(validationErrors) != 3 {
		t.Fatalf(
			"expected 3 validation errors, got %d: %v",
			len(validationErrors),
			validationErrors,
		)
	}
}

func TestValidate_MultipleRules(t *testing.T) {
	value := struct {
		Age int `validate:"min:18|max:50"`
	}{
		Age: 100,
	}

	err := Validate(value)

	var validationErrors ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}

	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(validationErrors))
	}

	if !errors.Is(validationErrors[0].Err, ErrGreaterThanMax) {
		t.Errorf(
			"expected ErrGreaterThanMax, got %v",
			validationErrors[0].Err,
		)
	}
}

func validUser() User {
	return User{
		ID:     "12345678-1234-1234-1234-123456789012",
		Age:    25,
		Email:  "user@example.com",
		Role:   "admin",
		Phones: []string{"79991234567"},
	}
}

func TestValidate_InvalidAge(t *testing.T) {
	user := validUser()
	user.Age = 10

	err := Validate(user)

	var validationErrors ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}

	if len(validationErrors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(validationErrors))
	}

	if validationErrors[0].Field != "Age" {
		t.Errorf("expected field Age, got %s", validationErrors[0].Field)
	}

	if !errors.Is(validationErrors[0].Err, ErrLessThanMin) {
		t.Errorf("expected ErrLessThanMin, got %v", validationErrors[0].Err)
	}
}
