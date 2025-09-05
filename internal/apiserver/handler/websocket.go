package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocketManager manages WebSocket connections for real-time updates
type WebSocketManager struct {
	logger      *zap.Logger
	connections map[string]*websocket.Conn
	mutex       sync.RWMutex
	upgrader    websocket.Upgrader
}

// SyncStatusMessage represents real-time sync status update
type SyncStatusMessage struct {
	Type      string `json:"type"`      // "sync_status"
	SyncID    string `json:"syncId"`
	Tenant    string `json:"tenant"`
	Server    string `json:"server"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp"`
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID         string
	Connection *websocket.Conn
	Tenant     string
	Server     string
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager(logger *zap.Logger) *WebSocketManager {
	return &WebSocketManager{
		logger:      logger.Named("websocket"),
		connections: make(map[string]*websocket.Conn),
		mutex:       sync.RWMutex{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// HandleSyncStatusWebSocket handles WebSocket connections for sync status updates
func (wsm *WebSocketManager) HandleSyncStatusWebSocket(c *gin.Context) {
	tenant := c.Param("tenant")
	server := c.Param("name")
	
	if tenant == "" || server == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant and server name required"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := wsm.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		wsm.logger.Error("failed to upgrade WebSocket connection", zap.Error(err))
		return
	}
	defer conn.Close()

	// Generate client ID
	clientID := tenant + "/" + server + "/" + time.Now().Format("20060102150405")
	
	// Store connection
	wsm.mutex.Lock()
	wsm.connections[clientID] = conn
	wsm.mutex.Unlock()

	// Remove connection on exit
	defer func() {
		wsm.mutex.Lock()
		delete(wsm.connections, clientID)
		wsm.mutex.Unlock()
		wsm.logger.Info("WebSocket client disconnected", zap.String("clientId", clientID))
	}()

	wsm.logger.Info("WebSocket client connected", zap.String("clientId", clientID), 
		zap.String("tenant", tenant), zap.String("server", server))

	// Send initial status
	initialMsg := SyncStatusMessage{
		Type:      "connection",
		SyncID:    "",
		Tenant:    tenant,
		Server:    server,
		Status:    "connected",
		Progress:  0,
		Message:   "WebSocket connected successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	
	if err := conn.WriteJSON(initialMsg); err != nil {
		wsm.logger.Error("failed to send initial message", zap.Error(err))
		return
	}

	// Handle ping/pong to keep connection alive
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Keep connection alive and handle close
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wsm.logger.Error("WebSocket connection error", zap.Error(err))
			}
			break
		}
		
		// Handle ping messages
		if messageType == websocket.PingMessage {
			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				wsm.logger.Error("failed to send pong", zap.Error(err))
				break
			}
		}
	}
}

// BroadcastSyncStatus broadcasts sync status updates to matching WebSocket connections
func (wsm *WebSocketManager) BroadcastSyncStatus(ctx context.Context, syncID, tenant, server, status string, progress int, message string) {
	msg := SyncStatusMessage{
		Type:      "sync_status",
		SyncID:    syncID,
		Tenant:    tenant,
		Server:    server,
		Status:    status,
		Progress:  progress,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	wsm.mutex.RLock()
	defer wsm.mutex.RUnlock()

	for clientID, conn := range wsm.connections {
		// Check if client is interested in this server
		if len(clientID) > 0 {
			parts := clientID[:len(clientID)-15] // Remove timestamp suffix
			if parts == tenant+"/"+server {
				// Send message to matching client
				if err := conn.WriteJSON(msg); err != nil {
					wsm.logger.Error("failed to send WebSocket message", 
						zap.String("clientId", clientID), zap.Error(err))
					// Remove failed connection
					conn.Close()
					delete(wsm.connections, clientID)
				}
			}
		}
	}

	wsm.logger.Debug("broadcasted sync status", 
		zap.String("syncId", syncID), 
		zap.String("tenant", tenant), 
		zap.String("server", server), 
		zap.String("status", status), 
		zap.Int("progress", progress))
}

// SendHeartbeat sends periodic heartbeat messages to all connections
func (wsm *WebSocketManager) SendHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			wsm.sendPingToAll()
		}
	}
}

// sendPingToAll sends ping to all connections
func (wsm *WebSocketManager) sendPingToAll() {
	wsm.mutex.RLock()
	connections := make(map[string]*websocket.Conn)
	for id, conn := range wsm.connections {
		connections[id] = conn
	}
	wsm.mutex.RUnlock()

	for clientID, conn := range connections {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			wsm.logger.Debug("ping failed, removing connection", 
				zap.String("clientId", clientID), zap.Error(err))
			
			wsm.mutex.Lock()
			conn.Close()
			delete(wsm.connections, clientID)
			wsm.mutex.Unlock()
		}
	}
}

// GetConnectionCount returns the number of active WebSocket connections
func (wsm *WebSocketManager) GetConnectionCount() int {
	wsm.mutex.RLock()
	defer wsm.mutex.RUnlock()
	return len(wsm.connections)
}

// CloseAll closes all WebSocket connections
func (wsm *WebSocketManager) CloseAll() {
	wsm.mutex.Lock()
	defer wsm.mutex.Unlock()
	
	for clientID, conn := range wsm.connections {
		conn.Close()
		wsm.logger.Info("closed WebSocket connection", zap.String("clientId", clientID))
	}
	wsm.connections = make(map[string]*websocket.Conn)
}