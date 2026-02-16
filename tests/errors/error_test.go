package errors_test

import (
	"errors"
	"testing"

	. "fuzoj/pkg/errors"
)

func TestErrorCode_Message(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{Success, "Success"},
		{UserNotFound, "User not found"},
		{InvalidParams, "Invalid parameters"},
		{DatabaseError, "Database operation failed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.code.Message(); got != tt.want {
				t.Errorf("Message() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorCode_HTTPStatus(t *testing.T) {
	tests := []struct {
		code       ErrorCode
		wantStatus int
	}{
		{Success, 200},
		{InvalidParams, 400},
		{Unauthorized, 401},
		{Forbidden, 403},
		{NotFound, 404},
		{TooManyRequests, 429},
		{InternalServerError, 500},
	}

	for _, tt := range tests {
		t.Run(tt.code.Message(), func(t *testing.T) {
			if got := tt.code.HTTPStatus(); got != tt.wantStatus {
				t.Errorf("HTTPStatus() = %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

func TestNew(t *testing.T) {
	err := New(UserNotFound)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != UserNotFound {
		t.Errorf("Code = %v, want %v", err.Code, UserNotFound)
	}

	if err.Error() != UserNotFound.Message() {
		t.Errorf("Error() = %v, want %v", err.Error(), UserNotFound.Message())
	}
}

func TestNewf(t *testing.T) {
	userID := int64(123)
	err := Newf(UserNotFound, "user %d not found", userID)

	want := "user 123 not found"
	if err.Error() != want {
		t.Errorf("Error() = %v, want %v", err.Error(), want)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("connection refused")
	wrappedErr := Wrap(originalErr, DatabaseError)

	if wrappedErr.Code != DatabaseError {
		t.Errorf("Code = %v, want %v", wrappedErr.Code, DatabaseError)
	}

	if wrappedErr.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}
}

func TestError_WithDetail(t *testing.T) {
	err := New(ValidationFailed).
		WithDetail("field", "email").
		WithDetail("reason", "invalid format")

	if err.Details["field"] != "email" {
		t.Error("Field detail not set correctly")
	}

	if err.Details["reason"] != "invalid format" {
		t.Error("Reason detail not set correctly")
	}
}

func TestError_WithMessage(t *testing.T) {
	customMsg := "custom error message"
	err := New(InternalServerError).WithMessage(customMsg)

	if err.Error() != customMsg {
		t.Errorf("Error() = %v, want %v", err.Error(), customMsg)
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{
			name: "nil error",
			err:  nil,
			want: Success,
		},
		{
			name: "custom error",
			err:  New(UserNotFound),
			want: UserNotFound,
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: InternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCode(tt.err); got != tt.want {
				t.Errorf("GetCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIs(t *testing.T) {
	err := New(UserNotFound)

	if !Is(err, UserNotFound) {
		t.Error("Is() should return true for matching code")
	}

	if Is(err, DatabaseError) {
		t.Error("Is() should return false for non-matching code")
	}

	if Is(nil, UserNotFound) {
		t.Error("Is() should return false for nil error")
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	t.Run("BadRequest", func(t *testing.T) {
		err := BadRequest("invalid input")
		if err.Code != InvalidParams {
			t.Error("BadRequest should use InvalidParams code")
		}
	})

	t.Run("NotFoundError", func(t *testing.T) {
		err := NotFoundError("user")
		if err.Code != NotFound {
			t.Error("NotFoundError should use NotFound code")
		}
	})

	t.Run("UnauthorizedError", func(t *testing.T) {
		err := UnauthorizedError("token expired")
		if err.Code != Unauthorized {
			t.Error("UnauthorizedError should use Unauthorized code")
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		originalErr := errors.New("db error")
		err := InternalError(originalErr)
		if err.Code != InternalServerError {
			t.Error("InternalError should use InternalServerError code")
		}
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError("email", "invalid format")
		if err.Code != ValidationFailed {
			t.Error("ValidationError should use ValidationFailed code")
		}
		if err.Details["field"] != "email" {
			t.Error("Field detail not set")
		}
	})
}
