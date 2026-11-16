import { AlertCircle } from 'lucide-react';
import { Button } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

interface FormErrorAlertProps {
  message: string;
  /** When there is something to run again, the alert offers it. */
  onRetry?: () => void;
  retryDisabled?: boolean;
}

/**
 * Stays on screen until the person acts, unlike a toast: the data they typed
 * survived, and "Reintentar" resubmits it as-is instead of asking them to
 * start over. Disabled while that resubmit runs. The same box for a booking, an
 * event and a tournament.
 */
export function FormErrorAlert({ message, onRetry, retryDisabled = false }: FormErrorAlertProps) {
  return (
    <div
      role="alert"
      className="border-error-border/30 bg-error-bg text-error-text flex flex-col items-start gap-3 rounded-xl border p-4 text-sm sm:flex-row sm:items-center sm:justify-between"
    >
      <span className="flex items-start gap-2">
        <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
        {message}
      </span>
      {onRetry && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="border-error-border/30 text-error-text hover:bg-error-bg/50 min-h-12 w-full shrink-0 rounded-xl sm:min-h-0 sm:w-auto"
          disabled={retryDisabled}
          onClick={onRetry}
        >
          {t.publicBooking.retryPaymentLink}
        </Button>
      )}
    </div>
  );
}
