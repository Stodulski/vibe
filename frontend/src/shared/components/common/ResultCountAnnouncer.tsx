import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ResultCountAnnouncerProps {
  /**
   * How many rows the list is showing, or `null` while there is no settled
   * answer yet (the query is loading, or it failed) — the region stays
   * mounted either way, it just has nothing to say.
   */
  count: number | null;
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
 * Mount it beside the list *container*, never inside the branch that draws
 * the rows: a region added to the page at the same time as its text is not
 * reliably announced, and an empty result — the one a person filtering most
 * needs to hear — renders a different branch entirely, so from in there
 * "0 resultados" would never be spoken at all. Pass `null` for the states
 * that have no count yet, so the region is present from the first render
 * without announcing a zero the query has not actually returned.
 */
export function ResultCountAnnouncer({ count }: ResultCountAnnouncerProps) {
  return (
    <p className="sr-only" aria-live="polite">
      {count === null ? '' : `${String(count)} ${count === 1 ? t.common.result : t.common.results}`}
    </p>
  );
}
