import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function NotesSection({ notes }: { notes: string }) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wider text-text-tertiary">{t.bookings.notes}</h3>
      <p className="break-words text-sm leading-relaxed text-text-secondary">{notes}</p>
    </div>
  );
}
