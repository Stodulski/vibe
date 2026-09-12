import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface RefundExpiredNoticeProps {
  cancellationHours: number | undefined;
}

export function RefundExpiredNotice({ cancellationHours }: RefundExpiredNoticeProps) {
  return (
    <div className="rounded-2xl border border-warning-border/30 bg-warning-bg/50 p-4 text-left text-sm">
      <p className="font-medium text-warning-text">{t.publicBooking.refundExpired}</p>
      <p className="mt-1 text-xs text-text-secondary">
        {t.publicBooking.refundExpiredBefore} <strong className="text-text-primary">{cancellationHours}h</strong>{' '}
        {t.publicBooking.refundExpiredAfter}{' '}
        <strong className="text-text-primary">{t.publicBooking.noRefundWarning}</strong>.
      </p>
    </div>
  );
}
