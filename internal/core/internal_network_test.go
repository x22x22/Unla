package core

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateToolEndpoint_InternalBlocked(t *testing.T) {
	s := &Server{}
	u, err := url.Parse("http://127.0.0.1:8080")
	assert.NoError(t, err)
	assert.Error(t, s.validateToolEndpoint(context.Background(), u))
}

func TestValidateToolEndpoint_InternalAllowlistedCIDR(t *testing.T) {
	allowlist, invalid := parseInternalNetworkAllowlist([]string{"127.0.0.0/8"})
	assert.Empty(t, invalid)

	s := &Server{internalNetACL: allowlist}
	u, err := url.Parse("http://127.0.0.1:8080")
	assert.NoError(t, err)
	assert.NoError(t, s.validateToolEndpoint(context.Background(), u))
}

func TestValidateToolEndpoint_InternalAllowlistedHost(t *testing.T) {
	allowlist, invalid := parseInternalNetworkAllowlist([]string{"internal.local"})
	assert.Empty(t, invalid)

	s := &Server{internalNetACL: allowlist}
	u, err := url.Parse("http://internal.local/health")
	assert.NoError(t, err)
	assert.NoError(t, s.validateToolEndpoint(context.Background(), u))
}

func TestValidateToolEndpoint_PublicIPAllowed(t *testing.T) {
	s := &Server{}
	u, err := url.Parse("http://8.8.8.8")
	assert.NoError(t, err)
	assert.NoError(t, s.validateToolEndpoint(context.Background(), u))
}
