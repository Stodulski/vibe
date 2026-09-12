import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface RefundExpiredNoticeProps {
  cancellationHours: number | undefined;
}

export function RefundExpiredNotice({ cancellationHours }: RefundExpiredNoticeProps) {
  return (
    <div className="border-warning-border/30 bg-warning-bg/50 rounded-2xl border p-4 text-left text-sm">
      <p className="text-warning-text font-medium">{t.publicBooking.refundExpired}</p>
      <p className="text-text-secondary mt-1 text-xs">
        {t.publicBooking.refundExpiredBefore} <strong className="text-text-primary">{cancellationHours}h</strong>{' '}
        {t.publicBooking.refundExpiredAfter}{' '}
        <strong className="text-text-primary">{t.publicBooking.noRefundWarning}</strong>.
      </p>
    </div>
  );
}
