import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function NotesSection({ notes }: { notes: string }) {
  return (
    <div className="space-y-2">
      <h3 className="text-text-tertiary text-xs font-semibold tracking-wider uppercase">{t.bookings.notes}</h3>
      <p className="text-text-secondary text-sm leading-relaxed break-words">{notes}</p>
    </div>
  );
}
