import { AGENDA_LIST_CLASS } from '@/shared/components/common/agendaRowLayout';
import { AgendaRowSkeleton } from '@/shared/components/common/AgendaRowSkeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

/** Loading shape of the events list on the events page: four rows of the one list. The one `role="status"` of the page body. */
export function EventListSkeleton() {
  return (
    <div role="status" aria-label={ES_AR.common.loading} className={AGENDA_LIST_CLASS}>
      {Array.from({ length: 4 }, (_, i) => (
        <AgendaRowSkeleton key={i} />
      ))}
    </div>
  );
}
