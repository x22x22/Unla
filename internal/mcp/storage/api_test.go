package storage

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAPIStore_GetAndList(t *testing.T) {
	// Serve a single MCPConfig JSON
	single := `{"name":"demo","tenant":"t","routers":[],"servers":[],"tools":[],"prompts":[],"mcpServers":[]}`
	arr := `[` + single + `]`
	mux := http.NewServeMux()
	mux.HandleFunc("/one", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(single)) })
	mux.HandleFunc("/many", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(arr)) })
	mux.HandleFunc("/wrap", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":` + single + `}`)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, err := NewAPIStore(zap.NewNop(), srv.URL+"/one", "", 2*time.Second)
	assert.NoError(t, err)
	cfg, err := s.Get(nil, "t", "demo")
	assert.NoError(t, err)
	assert.Equal(t, "demo", cfg.Name)

	s2, err := NewAPIStore(zap.NewNop(), srv.URL+"/many", "", 2*time.Second)
	assert.NoError(t, err)
	list, err := s2.List(nil)
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	// JSONPath extraction
	s3, err := NewAPIStore(zap.NewNop(), srv.URL+"/wrap", "data", 2*time.Second)
	assert.NoError(t, err)
	cfg2, err := s3.Get(nil, "t", "demo")
	assert.NoError(t, err)
	assert.Equal(t, "demo", cfg2.Name)

	// Missing path -> error
	s4, err := NewAPIStore(zap.NewNop(), srv.URL+"/wrap", "missing", 2*time.Second)
	assert.NoError(t, err)
	_, err = s4.List(nil)
	assert.Error(t, err)

	_ = config.APIStorageConfig{} // silence import in case of future edits
}
