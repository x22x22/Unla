package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/apiserver/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type chatDBMock struct {
	// configurable responses
	sessions            []*database.Session
	sessionsErr         error
	msgs                []*database.Message
	msgsErr             error
	sessionExists       bool
	sessionExistsErr    error
	createSessionErr    error
	createSessionTitle  string
	createSessionCalled bool
	createWithTitleErr  error
	saveMsgErr          error
	updateTitleErr      error
	deleteSessionErr    error
}

func (m *chatDBMock) Close() error { return nil }
func (m *chatDBMock) SaveMessage(ctx context.Context, message *database.Message) error {
	return m.saveMsgErr
}
func (m *chatDBMock) GetMessages(ctx context.Context, sessionID string) ([]*database.Message, error) {
	return nil, nil
}
func (m *chatDBMock) GetMessagesWithPagination(ctx context.Context, sessionID string, page, pageSize int) ([]*database.Message, error) {
	return m.msgs, m.msgsErr
}
func (m *chatDBMock) CreateSession(ctx context.Context, sessionId string) error {
	m.createSessionCalled = true
	return m.createSessionErr
}
func (m *chatDBMock) CreateSessionWithTitle(ctx context.Context, sessionId string, title string) error {
	m.createSessionTitle = title
	return m.createWithTitleErr
}
func (m *chatDBMock) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return m.sessionExists, m.sessionExistsErr
}
func (m *chatDBMock) GetSessions(ctx context.Context) ([]*database.Session, error) {
	return m.sessions, m.sessionsErr
}
func (m *chatDBMock) UpdateSessionTitle(ctx context.Context, sessionID string, title string) error {
	return m.updateTitleErr
}
func (m *chatDBMock) DeleteSession(ctx context.Context, sessionID string) error {
	return m.deleteSessionErr
}
func (m *chatDBMock) CreateUser(context.Context, *database.User) error { return nil }
func (m *chatDBMock) GetUserByUsername(context.Context, string) (*database.User, error) {
	return nil, nil
}
func (m *chatDBMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (m *chatDBMock) DeleteUser(context.Context, uint) error               { return nil }
func (m *chatDBMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (m *chatDBMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (m *chatDBMock) GetTenantByName(context.Context, string) (*database.Tenant, error) {
	return nil, nil
}
func (m *chatDBMock) GetTenantByID(context.Context, uint) (*database.Tenant, error) { return nil, nil }
func (m *chatDBMock) UpdateTenant(context.Context, *database.Tenant) error          { return nil }
func (m *chatDBMock) DeleteTenant(context.Context, uint) error                      { return nil }
func (m *chatDBMock) ListTenants(context.Context) ([]*database.Tenant, error)       { return nil, nil }
func (m *chatDBMock) AddUserToTenant(context.Context, uint, uint) error             { return nil }
func (m *chatDBMock) RemoveUserFromTenant(context.Context, uint, uint) error        { return nil }
func (m *chatDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) {
	return nil, nil
}
func (m *chatDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error) { return nil, nil }
func (m *chatDBMock) DeleteUserTenants(context.Context, uint) error                  { return nil }
func (m *chatDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *chatDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (m *chatDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

func TestChat_HandleGetChatSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &chatDBMock{sessions: []*database.Session{{ID: "s1", Title: "t", CreatedAt: time.Now()}}}
	h := NewChat(db, zap.NewNop())

	r := gin.New()
	r.GET("/sessions", h.HandleGetChatSessions)

	// success
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sessions", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// error path
	db.sessionsErr = context.DeadlineExceeded
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/sessions", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w2.Code)
	}
}

func TestChat_HandleGetChatMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &chatDBMock{msgs: []*database.Message{{ID: "m1", SessionID: "s1", Sender: "user", Timestamp: time.Now()}}}
	h := NewChat(db, zap.NewNop())

	r := gin.New()
	r.GET("/messages", h.HandleGetChatMessages)                     // missing sessionId
	r.GET("/sessions/:sessionId/messages", h.HandleGetChatMessages) // normal

	// missing sessionId -> 400
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/messages", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// success with invalid query values (to cover branches)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/sessions/s1/messages?page=abc&pageSize=xyz", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	// error from DB
	db.msgsErr = context.Canceled
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/sessions/s1/messages", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w3.Code)
	}
}

func TestChat_HandleDeleteChatSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &chatDBMock{}
	h := NewChat(db, zap.NewNop())

	r := gin.New()
	r.DELETE("/delete", h.HandleDeleteChatSession)              // missing id
	r.DELETE("/sessions/:sessionId", h.HandleDeleteChatSession) // normal

	// missing id
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/delete", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// success
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/sessions/s1", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	// error path
	db.deleteSessionErr = context.Canceled
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/sessions/s1", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w3.Code)
	}
}

func TestChat_HandleUpdateChatSessionTitle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &chatDBMock{}
	h := NewChat(db, zap.NewNop())

	r := gin.New()
	r.POST("/update", h.HandleUpdateChatSessionTitle) // missing id
	r.POST("/sessions/:sessionId/title", h.HandleUpdateChatSessionTitle)

	// missing id
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/update", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// invalid body
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/sessions/s1/title", bytes.NewReader([]byte("{}")))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w2.Code)
	}

	// success
	body, _ := json.Marshal(map[string]string{"title": "hello"})
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/sessions/s1/title", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w3.Code)
	}

	// error on update
	db.updateTitleErr = context.Canceled
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("POST", "/sessions/s1/title", bytes.NewReader(body))
	req4.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w4.Code)
	}
}

func TestChat_HandleSaveChatMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newRecorder := func() (*httptest.ResponseRecorder, *http.Request) {
		return httptest.NewRecorder(), nil
	}

	db := &chatDBMock{sessionExists: true}
	h := NewChat(db, zap.NewNop())
	r := gin.New()
	r.POST("/messages", h.HandleSaveChatMessage)

	// invalid body
	w, req := newRecorder()
	req, _ = http.NewRequest("POST", "/messages", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// no content
	payload := map[string]string{"id": "m1", "session_id": "s1", "sender": "user", "timestamp": time.Now().Format(time.RFC3339)}
	b, _ := json.Marshal(payload)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w2.Code)
	}

	// invalid timestamp
	payload["content"] = "hi"
	payload["timestamp"] = "bad"
	b, _ = json.Marshal(payload)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w3.Code)
	}

	// session exists -> save message success
	payload["timestamp"] = time.Now().Format(time.RFC3339)
	b, _ = json.Marshal(payload)
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req4.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w4.Code)
	}

	// save message error
	db.saveMsgErr = context.Canceled
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req5.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w5.Code)
	}

	// session not exists, user message with title -> CreateSessionWithTitle
	db.saveMsgErr = nil
	db.sessionExists = false
	longContent := make([]byte, 0)
	for i := 0; i < 60; i++ {
		longContent = append(longContent, 'a')
	}
	payload["content"] = string(longContent)
	b, _ = json.Marshal(payload)
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req6.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w6, req6)
	if w6.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w6.Code)
	}
	if db.createSessionTitle == "" {
		t.Fatalf("expected CreateSessionWithTitle to be called")
	}

	// session not exists, bot message -> CreateSession (no title).
	// Provide tool result to satisfy content validation.
	payload["sender"] = "bot"
	payload["content"] = ""
	payload["toolResult"] = "ok"
	b, _ = json.Marshal(payload)
	w7 := httptest.NewRecorder()
	req7, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req7.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w7, req7)
	if w7.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w7.Code)
	}
	if !db.createSessionCalled {
		t.Fatalf("expected CreateSession to be called")
	}

	// error on SessionExists
	db.sessionExistsErr = context.DeadlineExceeded
	payload["sender"] = "user"
	payload["content"] = "x"
	b, _ = json.Marshal(payload)
	w8 := httptest.NewRecorder()
	req8, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req8.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w8, req8)
	if w8.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w8.Code)
	}

	// error on CreateSessionWithTitle
	db.sessionExistsErr = nil
	db.sessionExists = false
	db.createWithTitleErr = context.Canceled
	b, _ = json.Marshal(payload)
	w9 := httptest.NewRecorder()
	req9, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req9.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w9, req9)
	if w9.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w9.Code)
	}

	// error on CreateSession
	db.createWithTitleErr = nil
	db.createSessionErr = context.Canceled
	payload["sender"] = "bot"
	payload["content"] = ""
	b, _ = json.Marshal(payload)
	w10 := httptest.NewRecorder()
	req10, _ := http.NewRequest("POST", "/messages", bytes.NewReader(b))
	req10.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w10, req10)
	if w10.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w10.Code)
	}
}
