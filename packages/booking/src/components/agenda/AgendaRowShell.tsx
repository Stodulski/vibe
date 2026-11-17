import { Fragment } from 'react';
import { Button, cn } from '@vibe/ui';
import { dayParts } from './agendaModel';

/** A token of the row's meta line: the hours, the places, the price. */
export interface AgendaMetaToken {
  key: string;
  text: string;
  /** Highlights the token, as the last places are. */
  emphasis?: boolean;
}

interface AgendaRowShellProps {
  /** The day the row starts (`YYYY-MM-DD`). */
  date: string;
  /** The small label above the title. */
  label: string;
  title: string;
  /** The meta line, in order. Its tokens are split by dots, which a phone hides. */
  meta: AgendaMetaToken[];
  viewDetailLabel: string;
  onViewDetail: () => void;
  signUp: { href: string; label: string };
}

/**
 * The date of a row, written as the agenda writes it: the weekday small, the day
 * number large in the accent, the month small. On a phone it is one compact line,
 * "MIÉ 18 NOV", above the detail, with no rule. From sm up it is a tall column
 * with a thin accent rule after it, to the left of the detail.
 */
function AgendaDateBlock({ date }: { date: string }) {
  const parts = dayParts(date);
  return (
    <div className="flex shrink-0 items-stretch gap-4">
      <time
        dateTime={date}
        className="flex flex-row items-baseline gap-2 sm:w-14 sm:flex-col sm:items-center sm:justify-center sm:gap-0 sm:text-center"
      >
        <span className="text-text-secondary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">
          {parts.weekday}
        </span>
        <span className="text-primary-400 text-2xl leading-none font-bold tabular-nums sm:text-3xl">{parts.day}</span>
        <span className="text-text-secondary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">
          {parts.month}
        </span>
      </time>
      <span aria-hidden="true" className="bg-primary-500 hidden w-px rounded-full sm:block" />
    </div>
  );
}

/**
 * One row of an agenda: its day, then its label and title, one meta line with
 * the detail action at its end, and its sign-up. On a phone the day sits above
 * the detail and the sign-up spans the full width below it; from sm up the day
 * sits to the left of the detail and the sign-up to the right. Each token of the
 * meta line keeps its words together, so a phone breaks the line between tokens
 * only.
 */
export function AgendaRowShell({ date, label, title, meta, viewDetailLabel, onViewDetail, signUp }: AgendaRowShellProps) {
  return (
    <article className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:gap-6">
      <div className="flex min-w-0 flex-1 flex-col gap-3 sm:flex-row sm:items-center sm:gap-5">
        <AgendaDateBlock date={date} />
        <div className="flex min-w-0 flex-col gap-1">
          <p className="text-text-tertiary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">{label}</p>
          <h3 className="text-text-primary text-lg leading-tight font-bold">{title}</h3>
          <p className="text-text-secondary flex flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm sm:gap-x-0">
            {meta.map((token, index) => (
              <Fragment key={token.key}>
                {index > 0 && (
                  <span aria-hidden="true" className="mx-1.5 hidden sm:inline">
                    ·
                  </span>
                )}
                <span className={cn('whitespace-nowrap', token.emphasis && 'text-warning-text font-medium')}>
                  {token.text}
                </span>
              </Fragment>
            ))}
            <span aria-hidden="true" className="mx-1.5 hidden sm:inline">
              ·
            </span>
            <button
              type="button"
              onClick={onViewDetail}
              className="text-primary-400 hover:text-primary-300 cursor-pointer font-medium whitespace-nowrap underline-offset-4 hover:underline"
            >
              {viewDetailLabel}
            </button>
          </p>
        </div>
      </div>

      <Button asChild size="lg" className="min-h-12 w-full sm:w-auto sm:shrink-0">
        <a href={signUp.href}>
          {signUp.label}
          <span className="sr-only">{`: ${title}`}</span>
        </a>
      </Button>
    </article>
  );
}
