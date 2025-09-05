import React from 'react';
import { Chip, Button, Card, CardBody, Progress } from '@heroui/react';
import { 
  WifiIcon, 
  ExclamationTriangleIcon,
  ArrowPathIcon,
  SignalIcon,
  NoSymbolIcon
} from '@heroicons/react/24/outline';
import { useTranslation } from 'react-i18next';
import { useNetworkStatus } from '../../hooks/useNetworkStatus';

interface NetworkStatusIndicatorProps {
  showDetails?: boolean;
  showReconnectButton?: boolean;
  position?: 'top' | 'bottom' | 'inline';
  className?: string;
}

export const NetworkStatusIndicator: React.FC<NetworkStatusIndicatorProps> = ({
  showDetails = false,
  showReconnectButton = true,
  position = 'top',
  className = '',
}) => {
  const { t } = useTranslation();
  const {
    isOnline,
    isConnecting,
    isReconnecting,
    connectionType,
    effectiveType,
    downlink,
    rtt,
    lastDisconnected,
    reconnect,
  } = useNetworkStatus();

  // Don't show indicator when online and not in details mode
  if (isOnline && !showDetails && !isConnecting) {
    return null;
  }

  const getStatusColor = () => {
    if (isConnecting || isReconnecting) return 'warning';
    return isOnline ? 'success' : 'danger';
  };

  const getStatusIcon = () => {
    if (isConnecting || isReconnecting) {
      return <ArrowPathIcon className="h-4 w-4 animate-spin" />;
    }
    
    if (!isOnline) {
      return <NoSymbolIcon className="h-4 w-4" />;
    }

    // Show different icons based on connection type and quality
    if (connectionType === 'wifi') {
      return <WifiIcon className="h-4 w-4" />;
    }
    
    if (connectionType === 'cellular') {
      return <SignalIcon className="h-4 w-4" />;
    }
    
    return <WifiIcon className="h-4 w-4" />;
  };

  const getStatusText = () => {
    if (isReconnecting) return t('network.reconnecting');
    if (isConnecting) return t('network.connecting');
    if (!isOnline) return t('network.offline');
    return t('network.online');
  };

  const getConnectionQuality = () => {
    if (!isOnline) return null;
    
    // Determine quality based on effective type and metrics
    if (effectiveType === '4g' && downlink > 1.5) return 'excellent';
    if (effectiveType === '4g' || (effectiveType === '3g' && downlink > 0.7)) return 'good';
    if (effectiveType === '3g' || effectiveType === '2g') return 'fair';
    return 'poor';
  };

  const getQualityColor = (quality: string | null) => {
    switch (quality) {
      case 'excellent': return 'success';
      case 'good': return 'primary';
      case 'fair': return 'warning';
      case 'poor': return 'danger';
      default: return 'default';
    }
  };

  const formatTime = (date: Date) => {
    return new Intl.DateTimeFormat('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }).format(date);
  };

  const getPositionClasses = () => {
    switch (position) {
      case 'top':
        return 'fixed top-4 right-4 z-50';
      case 'bottom':
        return 'fixed bottom-4 right-4 z-50';
      case 'inline':
      default:
        return '';
    }
  };

  if (showDetails) {
    return (
      <Card className={`${getPositionClasses()} ${className}`}>
        <CardBody className="p-4">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                {getStatusIcon()}
                <span className="font-medium">{t('network.status')}</span>
              </div>
              <Chip color={getStatusColor()} variant="flat" size="sm">
                {getStatusText()}
              </Chip>
            </div>

            {isOnline && (
              <>
                <div className="space-y-2">
                  <div className="flex justify-between text-sm">
                    <span>{t('network.connection_type')}</span>
                    <span className="capitalize">{connectionType}</span>
                  </div>
                  
                  <div className="flex justify-between text-sm">
                    <span>{t('network.speed')}</span>
                    <span className="capitalize">{effectiveType}</span>
                  </div>
                  
                  {downlink > 0 && (
                    <div className="flex justify-between text-sm">
                      <span>{t('network.downlink')}</span>
                      <span>{downlink.toFixed(1)} Mbps</span>
                    </div>
                  )}
                  
                  {rtt > 0 && (
                    <div className="flex justify-between text-sm">
                      <span>{t('network.latency')}</span>
                      <span>{rtt}ms</span>
                    </div>
                  )}

                  {/* Connection quality indicator */}
                  {getConnectionQuality() && (
                    <div className="space-y-1">
                      <div className="flex justify-between text-sm">
                        <span>{t('network.quality')}</span>
                        <span className={`capitalize text-${getQualityColor(getConnectionQuality())}`}>
                          {t(`network.quality.${getConnectionQuality()}`)}
                        </span>
                      </div>
                      <Progress 
                        value={
                          getConnectionQuality() === 'excellent' ? 100 :
                          getConnectionQuality() === 'good' ? 75 :
                          getConnectionQuality() === 'fair' ? 50 : 25
                        }
                        color={getQualityColor(getConnectionQuality()) as any}
                        size="sm"
                      />
                    </div>
                  )}
                </div>
              </>
            )}

            {!isOnline && lastDisconnected && (
              <div className="text-sm text-gray-500">
                {t('network.disconnected_at', { time: formatTime(lastDisconnected) })}
              </div>
            )}

            {(isReconnecting || (!isOnline && showReconnectButton)) && (
              <Button
                color="primary"
                size="sm"
                variant="flat"
                onPress={reconnect}
                disabled={isReconnecting}
                startContent={
                  isReconnecting ? 
                    <ArrowPathIcon className="h-4 w-4 animate-spin" /> : 
                    <ArrowPathIcon className="h-4 w-4" />
                }
                fullWidth
              >
                {isReconnecting ? t('network.reconnecting') : t('network.reconnect')}
              </Button>
            )}
          </div>
        </CardBody>
      </Card>
    );
  }

  // Compact indicator
  return (
    <div className={`${getPositionClasses()} ${className}`}>
      <Chip
        color={getStatusColor()}
        variant="flat"
        startContent={getStatusIcon()}
        size="sm"
        className="animate-in slide-in-from-top-2"
      >
        {getStatusText()}
      </Chip>
    </div>
  );
};

// Hook to provide network status context to components
export const useNetworkStatusContext = () => {
  const networkStatus = useNetworkStatus();
  
  return {
    ...networkStatus,
    isNetworkError: !networkStatus.isOnline,
    shouldShowOfflineMessage: !networkStatus.isOnline && !networkStatus.isReconnecting,
    shouldDisableActions: !networkStatus.isOnline || networkStatus.isConnecting,
  };
};

// Higher-order component to wrap components with network status awareness
export const withNetworkStatus = <P extends object>(
  Component: React.ComponentType<P>,
  options?: {
    showIndicator?: boolean;
    disableWhenOffline?: boolean;
    fallbackComponent?: React.ComponentType<any>;
  }
) => {
  const WrappedComponent = (props: P) => {
    const networkStatus = useNetworkStatus();
    const { showIndicator = true, disableWhenOffline = false, fallbackComponent: FallbackComponent } = options || {};

    if (!networkStatus.isOnline && FallbackComponent) {
      return <FallbackComponent networkStatus={networkStatus} {...props} />;
    }

    return (
      <div className="relative">
        {showIndicator && <NetworkStatusIndicator position="inline" />}
        <div className={disableWhenOffline && !networkStatus.isOnline ? 'pointer-events-none opacity-50' : ''}>
          <Component {...props} />
        </div>
      </div>
    );
  };

  WrappedComponent.displayName = `withNetworkStatus(${Component.displayName || Component.name})`;
  
  return WrappedComponent;
};

export default NetworkStatusIndicator;