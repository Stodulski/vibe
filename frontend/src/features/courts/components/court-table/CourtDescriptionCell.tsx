import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** Truncated, with the full text on `title`; the card's own empty-state wording when there is none. */
export function CourtDescriptionCell({ description }: { description: string | null | undefined }) {
  if (!description) return <p className="text-text-tertiary truncate text-sm">{t.courts.noDescription}</p>;
  return (
    <p className="text-text-secondary truncate text-sm" title={description}>
      {description}
    </p>
  );
}
