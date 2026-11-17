import { Fragment, type ReactNode } from 'react';
import { eventDayParts } from '@/shared/lib/eventDates';
import { cn } from '@/shared/lib/utils';

/** One token of a row's meta line: the hours, the places, the price, the courts. */
export interface AgendaRowToken {
  key: string;
  text: string;
  /** Capitalises the first letter, for a token that opens the line as a sentence. */
  capitalized?: boolean | undefined;
}

interface AgendaRowProps {
  /** The day the row starts (`YYYY-MM-DD`). */
  date: string;
  /** The small uppercase line above the title. */
  kicker: string;
  title: string;
  /** A status word beside the title, in its own colour. */
  badge?: ReactNode;
  /** The meta line, in order. A phone hides the dots between tokens and wraps between them. */
  meta: AgendaRowToken[];
}

/**
 * The day of a row, written as the public agenda writes it: the weekday small, the
 * day number large in the accent, the month small. From sm up it is a tall column
 * with a thin accent rule after it; on a phone it is one compact line above the detail.
 */
function AgendaDate({ date }: { date: string }) {
  const parts = eventDayParts(date);
  return (
    <span className="flex shrink-0 items-stretch gap-4">
      <time
        dateTime={date}
        className="flex flex-row items-baseline gap-2 sm:w-14 sm:flex-col sm:items-center sm:justify-center sm:gap-0 sm:text-center"
      >
        <span className="text-text-secondary text-2xl leading-none font-bold whitespace-nowrap uppercase sm:text-xs sm:leading-normal sm:font-semibold sm:tracking-wider">
          {parts.weekday}
        </span>
        <span className="text-primary-400 text-2xl leading-none font-bold tabular-nums sm:text-3xl">{parts.day}</span>
        <span className="text-text-secondary text-2xl leading-none font-bold whitespace-nowrap uppercase sm:text-xs sm:leading-normal sm:font-semibold sm:tracking-wider">
          {parts.month}
        </span>
      </time>
      <span aria-hidden="true" className="bg-primary-500 hidden w-px rounded-full sm:block" />
    </span>
  );
}

/**
 * The content of one row of the events hub: its day, then the kicker, the title with
 * its status badge, and one meta line. It is phrasing content only, so the caller puts
 * it inside the one focusable element that makes the whole row pressable.
 */
export function AgendaRow({ date, kicker, title, badge, meta }: AgendaRowProps) {
  return (
    <>
      <AgendaDate date={date} />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-text-tertiary text-xs font-semibold tracking-wider uppercase">{kicker}</span>
        <span className="flex min-w-0 flex-wrap items-baseline gap-x-2">
          <span className="text-text-primary min-w-0 text-lg leading-tight font-bold break-words">{title}</span>
          {badge}
        </span>
        <span className="text-text-secondary flex flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm sm:gap-x-0">
          {meta.map((token, index) => (
            <Fragment key={token.key}>
              {index > 0 && (
                <span aria-hidden="true" className="mx-1.5 hidden sm:inline">
                  ·
                </span>
              )}
              <span className={cn('whitespace-nowrap', token.capitalized && 'inline-block first-letter:uppercase')}>
                {token.text}
              </span>
            </Fragment>
          ))}
        </span>
      </span>
    </>
  );
}
