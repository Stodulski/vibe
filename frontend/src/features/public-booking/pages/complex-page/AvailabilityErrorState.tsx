import { AlertCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface AvailabilityErrorStateProps {
  onRetry: () => void;
}

/**
 * Inline error shown inside the availability grid when `useAvailability`
 * fails — a network drop or a 500. Kept inline (not a full-page `StatusHero`)
 * because the rest of the page (date picker, sport/duration steps) is still
 * usable; only this one section failed.
 */
export function AvailabilityErrorState({ onRetry }: AvailabilityErrorStateProps) {
  return (
    <div
      key="availability-error"
      role="alert"
      className="border-error-border/30 bg-error-bg text-error-text animate-fade-in flex flex-col items-center gap-3 rounded-xl border px-4 py-8 text-center text-sm"
    >
      <AlertCircle className="size-5" aria-hidden="true" />
      <p>{t.publicBooking.availabilityLoadError}</p>
      <Button
        variant="outline"
        size="sm"
        className="border-error-border/30 text-error-text hover:bg-error-bg/50 rounded-xl"
        onClick={onRetry}
      >
        {t.publicBooking.tryAgain}
      </Button>
    </div>
  );
}
