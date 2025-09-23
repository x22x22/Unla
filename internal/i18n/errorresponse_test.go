package i18n

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrorResponse_FactoryHelpers(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) *ErrorResponse
		code ErrorCode
	}{
		{"BadRequest", BadRequest, ErrorBadRequest},
		{"Unauthorized", Unauthorized, ErrorUnauthorized},
		{"Forbidden", Forbidden, ErrorForbidden},
		{"NotFound", NotFound, ErrorNotFound},
		{"Conflict", Conflict, ErrorConflict},
		{"InternalError", InternalError, ErrorInternalServer},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.fn("MsgID")
			assert.Equal(t, tc.code, r.StatusCode)
			// Err should be *ErrorWithCode holding the same code
			var ew *ErrorWithCode
			assert.True(t, errors.As(r.Err, &ew))
			if ew != nil {
				assert.Equal(t, tc.code, ew.GetCode())
			}
		})
	}
}

func TestErrorResponse_ErrorAndFrom(t *testing.T) {
	// From ErrorWithCode preserves code
	base := NewErrorWithCode("X", ErrorForbidden)
	r1 := Error(base)
	assert.Equal(t, ErrorForbidden, r1.StatusCode)
	assert.Equal(t, base, r1.Err)

	// From generic error yields InternalServer
	ge := errors.New("oops")
	r2 := Error(ge)
	assert.Equal(t, ErrorInternalServer, r2.StatusCode)
	assert.Equal(t, ge, r2.Err)

	// From is alias of Error
	r3 := From(base)
	assert.Equal(t, r1.StatusCode, r3.StatusCode)
}

func TestErrorResponse_FromErrVariants(t *testing.T) {
	// Table driven for both ErrorWithCode and generic error paths
	type variantFn func(error) *ErrorResponse
	table := []struct {
		name string
		fn   variantFn
		code ErrorCode
	}{
		{"NotFoundFromErr", NotFoundFromErr, ErrorNotFound},
		{"BadRequestFromErr", BadRequestFromErr, ErrorBadRequest},
		{"UnauthorizedFromErr", UnauthorizedFromErr, ErrorUnauthorized},
		{"ForbiddenFromErr", ForbiddenFromErr, ErrorForbidden},
		{"ConflictFromErr", ConflictFromErr, ErrorConflict},
		{"InternalServerFromErr", InternalServerFromErr, ErrorInternalServer},
	}

	for _, tc := range table {
		t.Run(tc.name+"_ErrWithCode", func(t *testing.T) {
			base := NewErrorWithCode("X", ErrorBadRequest)
			r := tc.fn(base)
			assert.Equal(t, tc.code, r.StatusCode)
			var ew *ErrorWithCode
			assert.True(t, errors.As(r.Err, &ew))
			if ew != nil {
				assert.Equal(t, tc.code, ew.GetCode())
			}
		})

		t.Run(tc.name+"_GenericErr", func(t *testing.T) {
			ge := errors.New("plain")
			r := tc.fn(ge)
			assert.Equal(t, tc.code, r.StatusCode)
			var ew *ErrorWithCode
			assert.True(t, errors.As(r.Err, &ew))
			if ew != nil {
				assert.Equal(t, tc.code, ew.GetCode())
			}
		})
	}
}

func TestErrorResponse_ParamsHelpers(t *testing.T) {
	// ErrorWithParam with ErrorWithCode preserves code and adds data
	base := NewErrorWithCode("X", ErrorUnauthorized)
	r1 := ErrorWithParam(base, "k", 1)
	var ew1 *ErrorWithCode
	assert.True(t, errors.As(r1.Err, &ew1))
	if ew1 != nil {
		assert.Equal(t, ErrorUnauthorized, r1.StatusCode)
		assert.Equal(t, any(1), ew1.GetData()["k"])
	}

	// ErrorWithParam with generic error returns internal with data
	r2 := ErrorWithParam(errors.New("e2"), "p", "v")
	var ew2 *ErrorWithCode
	assert.True(t, errors.As(r2.Err, &ew2))
	if ew2 != nil {
		assert.Equal(t, ErrorInternalServer, r2.StatusCode)
		assert.Equal(t, "v", ew2.GetData()["p"])
	}

	// ErrorWithParams adds multiple values
	r3 := ErrorWithParams(NewErrorWithCode("Y", ErrorConflict), map[string]any{"a": 1, "b": 2})
	var ew3 *ErrorWithCode
	assert.True(t, errors.As(r3.Err, &ew3))
	if ew3 != nil {
		assert.Equal(t, ErrorConflict, r3.StatusCode)
		assert.Equal(t, any(1), ew3.GetData()["a"])
		assert.Equal(t, any(2), ew3.GetData()["b"])
	}
}

func TestErrorResponse_MethodChainingAndSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Route 1: plain BadRequest().Send
	r.GET("/bad", func(c *gin.Context) {
		BadRequest("ErrBad").Send(c)
	})

	// Route 2: WithParam and WithHeader chaining
	r.GET("/unauth", func(c *gin.Context) {
		Unauthorized("ErrUnauth").WithParam("x", 1).WithHeader("X-Test", "1").Send(c)
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/bad", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusBadRequest, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/unauth", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestErrorResponse_WithHttpCode_FieldOnly(t *testing.T) {
	// Ensure WithHttpCode mutates the struct field as advertised
	er := BadRequest("E")
	assert.Equal(t, ErrorBadRequest, er.StatusCode)
	er.WithHttpCode(ErrorForbidden)
	assert.Equal(t, ErrorForbidden, er.StatusCode)
}

func TestErrorWithParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with ErrorWithCode", func(t *testing.T) {
		// Create an ErrorWithCode
		baseErr := NewErrorWithCode("TestError", ErrorBadRequest)
		params := map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		}

		resp := ErrorWithParams(baseErr, params)

		assert.Equal(t, ErrorBadRequest, resp.StatusCode)
		var errWithCode *ErrorWithCode
		assert.True(t, errors.As(resp.Err, &errWithCode))
		assert.Equal(t, ErrorBadRequest, errWithCode.GetCode())

		// Check that parameters were applied
		data := errWithCode.GetData()
		assert.Equal(t, "value1", data["param1"])
		assert.Equal(t, "value2", data["param2"])
	})

	t.Run("with regular error", func(t *testing.T) {
		regularErr := errors.New("regular error")
		params := map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		}

		resp := ErrorWithParams(regularErr, params)

		assert.Equal(t, ErrorInternalServer, resp.StatusCode)
		var errWithCode *ErrorWithCode
		assert.True(t, errors.As(resp.Err, &errWithCode))
		assert.Equal(t, ErrorInternalServer, errWithCode.GetCode())

		// Check that parameters were applied
		data := errWithCode.GetData()
		assert.Equal(t, "value1", data["param1"])
		assert.Equal(t, "value2", data["param2"])
	})

	t.Run("with empty params", func(t *testing.T) {
		baseErr := NewErrorWithCode("TestError", ErrorForbidden)
		params := map[string]interface{}{}

		resp := ErrorWithParams(baseErr, params)

		assert.Equal(t, ErrorForbidden, resp.StatusCode)
		var errWithCode *ErrorWithCode
		assert.True(t, errors.As(resp.Err, &errWithCode))
		assert.Equal(t, ErrorForbidden, errWithCode.GetCode())

		// Check that no parameters were applied
		data := errWithCode.GetData()
		assert.Equal(t, 0, len(data))
	})
}
