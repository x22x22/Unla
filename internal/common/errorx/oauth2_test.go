package errorx

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Error_ErrorAndConvert(t *testing.T) {
	e := &OAuth2Error{ErrorType: "invalid_request", ErrorDescription: "bad", HTTPStatus: http.StatusBadRequest}
	s := e.Error()
	assert.Contains(t, s, "\"invalid_request\"")
	assert.Contains(t, s, "\"bad\"")

	// pass-through
	out := ConvertToOAuth2Error(e)
	assert.Equal(t, e, out)

	// wrap other error
	out2 := ConvertToOAuth2Error(errors.New("boom"))
	assert.Equal(t, "unknown_error", out2.ErrorType)
	assert.Equal(t, http.StatusInternalServerError, out2.HTTPStatus)
	assert.Equal(t, "boom", out2.ErrorDescription)
}
