import { ShieldCheck } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function InfoSection({ connected }: { connected: boolean }) {
  return (
    <div className="space-y-3">
      <p className="text-xs text-text-secondary">{connected ? t.serviceFee.ownerInfo : t.mp.connectDescription}</p>

      {/* A line of reassurance, not a control. Bordered, filled and in the
          brand colour, it wore the secondary button's whole costume while
          doing nothing when pressed. */}
      <p className="flex items-center gap-2 text-xs font-medium text-primary-400">
        <ShieldCheck className="size-3.5 shrink-0" />
        {t.serviceFee.zeroCost}
      </p>

      {!connected && <p className="text-micro text-text-tertiary">{t.serviceFee.ownerInfo}</p>}
    </div>
  );
}
