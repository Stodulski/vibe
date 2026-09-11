import { CalendarDays } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function NoBookingsEmptyState({ isPast, onOpenCreate }: { isPast: boolean; onOpenCreate: () => void }) {
  return (
    <EmptyState
      icon={CalendarDays}
      title={t.bookings.noBookings}
      description={isPast ? '' : t.bookings.noBookingsDescription}
      actionLabel={isPast ? undefined : t.bookings.create}
      onAction={isPast ? undefined : onOpenCreate}
      actionVariant="outline"
    />
  );
}
