import { Loader2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function LoadingState() {
  return (
    <div className="flex flex-col items-center gap-5 py-16">
      <div className="relative">
        <div className="absolute inset-0 animate-ping-limited rounded-full bg-primary-500/20" />
        <Loader2 className="relative size-12 animate-spin text-primary-400" />
      </div>
      <p className="text-sm text-text-secondary">{t.publicBooking.processingPayment}</p>
      <div className="relative h-1 w-48 overflow-hidden rounded-full bg-bg-elevated">
        <div className="absolute inset-y-0 left-0 rounded-full bg-primary-500/60 animate-progress-120s" />
      </div>
    </div>
  );
}
