import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ResultCountAnnouncerProps {
  /** How many rows the list is showing right now. */
  count: number;
}

/**
 * Says out loud how many rows a list currently holds.
 *
 * Lists that filter as you type and grow as you scroll change size with no
 * visible event to announce: sighted readers see rows appear, a screen
 * reader was told nothing at all (A11Y-07). A polite live region carrying
 * just the count turns both into one short announcement, without moving
 * focus or interrupting whatever is being read.
 *
 * It must be mounted before the count changes — a region added to the page
 * at the same time as its text is not reliably announced — so render it
 * alongside the list, not inside the branch that draws the rows.
 */
export function ResultCountAnnouncer({ count }: ResultCountAnnouncerProps) {
  return (
    <p className="sr-only" aria-live="polite">
      {`${String(count)} ${count === 1 ? t.common.result : t.common.results}`}
    </p>
  );
}
