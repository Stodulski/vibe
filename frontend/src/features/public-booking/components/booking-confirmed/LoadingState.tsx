import { Loader2 } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function LoadingState() {
  return (
    <div className="flex flex-col items-center gap-5 py-16">
      <div className="relative">
        <div className="animate-ping-limited bg-primary-500/20 absolute inset-0 rounded-full" />
        <Loader2 className="text-primary-400 relative size-12 animate-spin" />
      </div>
      <p className="text-text-secondary text-sm">{t.publicBooking.processingPayment}</p>
      <div className="bg-bg-elevated relative h-1 w-48 overflow-hidden rounded-full">
        <div className="bg-primary-500/60 animate-progress-120s absolute inset-y-0 left-0 rounded-full" />
      </div>
    </div>
  );
}
