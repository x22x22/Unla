import { useState, useEffect, useCallback, useRef } from 'react';

export interface NetworkStatus {
  isOnline: boolean;
  isConnecting: boolean;
  connectionType: 'unknown' | 'wifi' | 'cellular' | 'ethernet';
  effectiveType: 'slow-2g' | '2g' | '3g' | '4g' | 'unknown';
  downlink: number;
  rtt: number;
  lastConnected: Date | null;
  lastDisconnected: Date | null;
}

interface NetworkStatusHookReturn extends NetworkStatus {
  checkConnection: () => Promise<boolean>;
  reconnect: () => Promise<void>;
  isReconnecting: boolean;
}

// Extend Navigator interface for network information
declare global {
  interface Navigator {
    connection?: {
      effectiveType: '2g' | '3g' | '4g' | 'slow-2g';
      type: 'bluetooth' | 'cellular' | 'ethernet' | 'none' | 'other' | 'unknown' | 'wifi' | 'wimax';
      downlink: number;
      rtt: number;
      addEventListener: (event: string, callback: () => void) => void;
      removeEventListener: (event: string, callback: () => void) => void;
    };
    onLine: boolean;
  }
}

const DEFAULT_PING_ENDPOINT = '/api/health';
const RECONNECT_INTERVAL = 3000;
const MAX_RECONNECT_ATTEMPTS = 10;

export const useNetworkStatus = (): NetworkStatusHookReturn => {
  const [networkStatus, setNetworkStatus] = useState<NetworkStatus>(() => ({
    isOnline: typeof window !== 'undefined' ? navigator.onLine : true,
    isConnecting: false,
    connectionType: 'unknown',
    effectiveType: 'unknown',
    downlink: 0,
    rtt: 0,
    lastConnected: null,
    lastDisconnected: null,
  }));

  const [isReconnecting, setIsReconnecting] = useState(false);
  const reconnectAttempts = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>();
  const lastPingRef = useRef<Date>(new Date());

  // Get network connection information from browser API
  const getConnectionInfo = useCallback(() => {
    const connection = navigator.connection;
    if (connection) {
      return {
        connectionType: connection.type || 'unknown',
        effectiveType: connection.effectiveType || 'unknown',
        downlink: connection.downlink || 0,
        rtt: connection.rtt || 0,
      };
    }
    return {
      connectionType: 'unknown' as const,
      effectiveType: 'unknown' as const,
      downlink: 0,
      rtt: 0,
    };
  }, []);

  // Ping server to check actual connectivity
  const checkConnection = useCallback(async (): Promise<boolean> => {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000);

      const response = await fetch(DEFAULT_PING_ENDPOINT, {
        method: 'HEAD',
        cache: 'no-cache',
        signal: controller.signal,
        headers: {
          'Cache-Control': 'no-cache',
          'Pragma': 'no-cache',
        },
      });

      clearTimeout(timeoutId);
      lastPingRef.current = new Date();
      
      return response.ok;
    } catch (error) {
      console.warn('Network connectivity check failed:', error);
      return false;
    }
  }, []);

  // Update network status
  const updateNetworkStatus = useCallback(async (isOnline: boolean) => {
    const connectionInfo = getConnectionInfo();
    const timestamp = new Date();
    
    // If coming back online, verify with server ping
    if (isOnline && !networkStatus.isOnline) {
      setNetworkStatus(prev => ({ ...prev, isConnecting: true }));
      
      const actuallyOnline = await checkConnection();
      
      setNetworkStatus(prev => ({
        ...prev,
        isOnline: actuallyOnline,
        isConnecting: false,
        ...connectionInfo,
        lastConnected: actuallyOnline ? timestamp : prev.lastConnected,
        lastDisconnected: actuallyOnline ? prev.lastDisconnected : timestamp,
      }));
      
      if (actuallyOnline) {
        reconnectAttempts.current = 0;
        setIsReconnecting(false);
      }
    } else {
      setNetworkStatus(prev => ({
        ...prev,
        isOnline,
        isConnecting: false,
        ...connectionInfo,
        lastConnected: isOnline ? (prev.lastConnected || timestamp) : prev.lastConnected,
        lastDisconnected: isOnline ? prev.lastDisconnected : timestamp,
      }));
    }
  }, [networkStatus.isOnline, getConnectionInfo, checkConnection]);

  // Auto-reconnect when offline
  const startReconnectProcess = useCallback(() => {
    if (isReconnecting || networkStatus.isOnline) {
      return;
    }

    setIsReconnecting(true);
    
    const attemptReconnect = async () => {
      if (reconnectAttempts.current >= MAX_RECONNECT_ATTEMPTS) {
        setIsReconnecting(false);
        return;
      }

      reconnectAttempts.current++;
      
      try {
        const isConnected = await checkConnection();
        
        if (isConnected) {
          await updateNetworkStatus(true);
          setIsReconnecting(false);
          reconnectAttempts.current = 0;
          return;
        }
      } catch (error) {
        console.warn('Reconnect attempt failed:', error);
      }

      // Schedule next attempt with exponential backoff
      const delay = Math.min(RECONNECT_INTERVAL * Math.pow(1.5, reconnectAttempts.current - 1), 30000);
      reconnectTimeoutRef.current = setTimeout(attemptReconnect, delay);
    };

    attemptReconnect();
  }, [isReconnecting, networkStatus.isOnline, checkConnection, updateNetworkStatus]);

  // Manual reconnect function
  const reconnect = useCallback(async () => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    
    reconnectAttempts.current = 0;
    setIsReconnecting(true);
    
    try {
      const isConnected = await checkConnection();
      await updateNetworkStatus(isConnected);
      
      if (!isConnected) {
        startReconnectProcess();
      }
    } catch (error) {
      console.error('Manual reconnect failed:', error);
      startReconnectProcess();
    }
  }, [checkConnection, updateNetworkStatus, startReconnectProcess]);

  // Event handlers
  const handleOnline = useCallback(() => {
    updateNetworkStatus(true);
  }, [updateNetworkStatus]);

  const handleOffline = useCallback(() => {
    updateNetworkStatus(false);
    // Start auto-reconnect process after a short delay
    setTimeout(startReconnectProcess, 1000);
  }, [updateNetworkStatus, startReconnectProcess]);

  const handleConnectionChange = useCallback(() => {
    const connectionInfo = getConnectionInfo();
    setNetworkStatus(prev => ({
      ...prev,
      ...connectionInfo,
    }));
  }, [getConnectionInfo]);

  // Set up event listeners
  useEffect(() => {
    if (typeof window === 'undefined') return;

    // Initial status update
    updateNetworkStatus(navigator.onLine);

    // Browser online/offline events
    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    // Network connection change events
    const connection = navigator.connection;
    if (connection) {
      connection.addEventListener('change', handleConnectionChange);
    }

    // Periodic connectivity check (every 30 seconds when online)
    const intervalId = setInterval(async () => {
      if (networkStatus.isOnline && Date.now() - lastPingRef.current.getTime() > 25000) {
        const isStillOnline = await checkConnection();
        if (!isStillOnline) {
          handleOffline();
        }
      }
    }, 30000);

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
      
      if (connection) {
        connection.removeEventListener('change', handleConnectionChange);
      }
      
      clearInterval(intervalId);
      
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [handleOnline, handleOffline, handleConnectionChange, networkStatus.isOnline, checkConnection]);

  return {
    ...networkStatus,
    checkConnection,
    reconnect,
    isReconnecting,
  };
};

export default useNetworkStatus;