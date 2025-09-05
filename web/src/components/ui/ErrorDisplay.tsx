import React from 'react';
import { Card, CardBody, Button, Chip, Link } from '@heroui/react';
import { 
  ExclamationTriangleIcon, 
  XCircleIcon, 
  InformationCircleIcon,
  ExclamationCircleIcon,
  ArrowPathIcon,
  QuestionMarkCircleIcon
} from '@heroicons/react/24/outline';
import { useTranslation } from 'react-i18next';

interface APIError {
  code: string;
  message: string;
  category: string;
  severity: 'info' | 'warning' | 'error' | 'critical';
  suggestions?: string[];
  actionable_steps?: ActionableStep[];
  trace_id?: string;
  timestamp?: string;
  help_url?: string;
}

interface ActionableStep {
  title: string;
  description: string;
  action?: string;
  url?: string;
}

interface ErrorDisplayProps {
  error: APIError | Error | string | null;
  title?: string;
  showRetry?: boolean;
  showDetails?: boolean;
  onRetry?: () => void;
  onDismiss?: () => void;
  className?: string;
  variant?: 'full' | 'compact' | 'minimal';
}

export const ErrorDisplay: React.FC<ErrorDisplayProps> = ({
  error,
  title,
  showRetry = true,
  showDetails = false,
  onRetry,
  onDismiss,
  className = '',
  variant = 'full',
}) => {
  const { t } = useTranslation();

  if (!error) return null;

  // Normalize error to APIError format
  const normalizedError: APIError = React.useMemo(() => {
    if (typeof error === 'string') {
      return {
        code: 'E5001',
        message: error,
        category: 'internal',
        severity: 'error',
      };
    }
    
    if (error instanceof Error) {
      return {
        code: 'E5001',
        message: error.message,
        category: 'internal',
        severity: 'error',
      };
    }
    
    return error as APIError;
  }, [error]);

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'info':
        return <InformationCircleIcon className="h-5 w-5 text-blue-500" />;
      case 'warning':
        return <ExclamationTriangleIcon className="h-5 w-5 text-yellow-500" />;
      case 'error':
        return <XCircleIcon className="h-5 w-5 text-red-500" />;
      case 'critical':
        return <ExclamationCircleIcon className="h-5 w-5 text-red-600" />;
      default:
        return <XCircleIcon className="h-5 w-5 text-red-500" />;
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'info':
        return 'primary';
      case 'warning':
        return 'warning';
      case 'error':
        return 'danger';
      case 'critical':
        return 'danger';
      default:
        return 'danger';
    }
  };

  if (variant === 'minimal') {
    return (
      <div className={`flex items-center gap-2 p-2 text-sm ${className}`}>
        {getSeverityIcon(normalizedError.severity)}
        <span className="text-gray-700 dark:text-gray-300">
          {normalizedError.message}
        </span>
        {onRetry && (
          <Button
            isIconOnly
            size="sm"
            variant="light"
            onPress={onRetry}
          >
            <ArrowPathIcon className="h-4 w-4" />
          </Button>
        )}
      </div>
    );
  }

  if (variant === 'compact') {
    return (
      <Card className={`border-l-4 border-l-${getSeverityColor(normalizedError.severity)} ${className}`}>
        <CardBody className="py-3">
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-2">
              {getSeverityIcon(normalizedError.severity)}
              <div>
                <p className="text-sm font-medium">{normalizedError.message}</p>
                {normalizedError.suggestions && normalizedError.suggestions.length > 0 && (
                  <p className="text-xs text-gray-600 dark:text-gray-400 mt-1">
                    {normalizedError.suggestions[0]}
                  </p>
                )}
              </div>
            </div>
            <div className="flex gap-1">
              {onRetry && (
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={onRetry}
                >
                  <ArrowPathIcon className="h-4 w-4" />
                </Button>
              )}
              {onDismiss && (
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={onDismiss}
                >
                  <XCircleIcon className="h-4 w-4" />
                </Button>
              )}
            </div>
          </div>
        </CardBody>
      </Card>
    );
  }

  return (
    <Card className={`${className}`}>
      <CardBody className="p-6">
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            {getSeverityIcon(normalizedError.severity)}
          </div>
          
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-4 mb-3">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                  {title || t('error.title')}
                </h3>
                {normalizedError.code && (
                  <div className="flex items-center gap-2 mt-1">
                    <Chip
                      size="sm"
                      color={getSeverityColor(normalizedError.severity)}
                      variant="flat"
                    >
                      {normalizedError.code}
                    </Chip>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      {normalizedError.category}
                    </span>
                  </div>
                )}
              </div>
              
              {onDismiss && (
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={onDismiss}
                  className="text-gray-400 hover:text-gray-600"
                >
                  <XCircleIcon className="h-4 w-4" />
                </Button>
              )}
            </div>

            <p className="text-gray-700 dark:text-gray-300 mb-4">
              {normalizedError.message}
            </p>

            {normalizedError.suggestions && normalizedError.suggestions.length > 0 && (
              <div className="mb-4">
                <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">
                  {t('error.suggestions')}
                </h4>
                <ul className="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                  {normalizedError.suggestions.map((suggestion, index) => (
                    <li key={index} className="flex items-start gap-2">
                      <span className="text-primary font-medium">•</span>
                      <span>{suggestion}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {normalizedError.actionable_steps && normalizedError.actionable_steps.length > 0 && (
              <div className="mb-4">
                <h4 className="text-sm font-medium text-gray-900 dark:text-gray-100 mb-3">
                  {t('error.actionable_steps')}
                </h4>
                <div className="space-y-3">
                  {normalizedError.actionable_steps.map((step, index) => (
                    <div key={index} className="flex items-start gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex-shrink-0 w-6 h-6 bg-primary text-primary-foreground rounded-full flex items-center justify-center text-xs font-bold">
                        {index + 1}
                      </div>
                      <div className="flex-1">
                        <h5 className="font-medium text-sm text-gray-900 dark:text-gray-100 mb-1">
                          {step.title}
                        </h5>
                        <p className="text-sm text-gray-600 dark:text-gray-400">
                          {step.description}
                        </p>
                        {step.url && (
                          <Link
                            href={step.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs mt-2 inline-flex items-center gap-1"
                          >
                            Learn more
                            <QuestionMarkCircleIcon className="h-3 w-3" />
                          </Link>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div className="flex items-center justify-between gap-4">
              <div className="flex gap-3">
                {showRetry && onRetry && (
                  <Button
                    color="primary"
                    variant="solid"
                    size="sm"
                    onPress={onRetry}
                    startContent={<ArrowPathIcon className="h-4 w-4" />}
                  >
                    {t('error.retry')}
                  </Button>
                )}
                
                {normalizedError.help_url && (
                  <Button
                    as={Link}
                    href={normalizedError.help_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    color="default"
                    variant="bordered"
                    size="sm"
                    startContent={<QuestionMarkCircleIcon className="h-4 w-4" />}
                  >
                    {t('error.get_help')}
                  </Button>
                )}
              </div>

              {showDetails && normalizedError.trace_id && (
                <details className="text-xs">
                  <summary className="cursor-pointer text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300">
                    {t('error.technical_details')}
                  </summary>
                  <div className="mt-2 p-2 bg-gray-100 dark:bg-gray-800 rounded text-gray-600 dark:text-gray-400">
                    <p><strong>Trace ID:</strong> {normalizedError.trace_id}</p>
                    {normalizedError.timestamp && (
                      <p><strong>Timestamp:</strong> {normalizedError.timestamp}</p>
                    )}
                  </div>
                </details>
              )}
            </div>
          </div>
        </div>
      </CardBody>
    </Card>
  );
};

// Hook to handle API errors consistently
export const useErrorHandler = () => {
  const handleError = React.useCallback((error: any): APIError => {
    // Handle fetch errors
    if (error.name === 'TypeError' && error.message.includes('fetch')) {
      return {
        code: 'E5031',
        message: 'Network connection failed',
        category: 'network',
        severity: 'error',
        suggestions: [
          'Check your internet connection',
          'Try refreshing the page',
          'Contact support if the problem persists'
        ],
      };
    }

    // Handle API error responses
    if (error.response && error.response.data && error.response.data.error) {
      return error.response.data.error;
    }

    // Handle generic errors
    return {
      code: 'E5001',
      message: error.message || 'An unexpected error occurred',
      category: 'internal',
      severity: 'error',
      suggestions: [
        'Please try again',
        'If the problem persists, contact support'
      ],
    };
  }, []);

  return { handleError };
};

export default ErrorDisplay;