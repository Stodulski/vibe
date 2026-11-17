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
  /** The hours. A phone shows them at the right of the date; from sm up they open the meta line. */
  time?: string | undefined;
  /** The meta line after the hours, in order. Its tokens are split by dots, which a phone hides. */
  meta: AgendaMetaToken[];
  viewDetailLabel: string;
  onViewDetail: () => void;
  signUp: { href: string; label: string };
}

/**
 * The date of a row, written as the agenda writes it: the weekday small, the day
 * number large in the accent, the month small. On a phone it is one compact line,
 * "MIÉ 18 NOV", above the detail, with no rule. From sm up it is a tall column
 * with a thin accent rule after it, to the left of the detail. On a phone the hours
 * sit at the right end of that line.
 */
function AgendaDateBlock({ date, time }: { date: string; time: string | undefined }) {
  const parts = dayParts(date);
  return (
    <div className="flex shrink-0 items-stretch gap-4">
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
      {time && (
        <span className="text-text-primary ml-auto self-center text-base leading-none font-semibold whitespace-nowrap tabular-nums sm:hidden">
          {time}
        </span>
      )}
    </div>
  );
}

type MetaToken = AgendaMetaToken & { wideOnly?: boolean };

/** The meta line: its tokens split by dots from sm up, then the detail action. */
function AgendaMeta({
  tokens,
  viewDetailLabel,
  onViewDetail,
}: {
  tokens: MetaToken[];
  viewDetailLabel: string;
  onViewDetail: () => void;
}) {
  return (
    <p className="text-text-secondary flex flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm sm:gap-x-0">
      {tokens.map((token, index) => (
        <Fragment key={token.key}>
          {index > 0 && (
            <span aria-hidden="true" className="mx-1.5 hidden sm:inline">
              ·
            </span>
          )}
          <span
            className={cn(
              'whitespace-nowrap',
              token.emphasis && 'text-warning-text font-medium',
              token.wideOnly && 'hidden sm:inline',
            )}
          >
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
        className="text-primary-400 hover:text-primary-300 -my-2 inline-flex cursor-pointer items-center font-medium whitespace-nowrap underline-offset-4 hover:underline sm:my-0"
      >
        {viewDetailLabel}
      </button>
    </p>
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
export function AgendaRowShell({
  date,
  label,
  title,
  time,
  meta,
  viewDetailLabel,
  onViewDetail,
  signUp,
}: AgendaRowShellProps) {
  // The hours lead the meta line from sm up only: a phone already shows them beside the date.
  const tokens: MetaToken[] = time
    ? [{ key: 'time', text: time, wideOnly: true }, ...meta]
    : meta;
  return (
    <article className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:gap-6">
      <div className="flex min-w-0 flex-1 flex-col gap-3 sm:flex-row sm:items-center sm:gap-5">
        <AgendaDateBlock date={date} time={time} />
        <div className="flex min-w-0 flex-col gap-2 sm:gap-1">
          <p className="text-text-tertiary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">{label}</p>
          <h3 className="text-text-primary text-lg leading-tight font-bold">{title}</h3>
          <AgendaMeta tokens={tokens} viewDetailLabel={viewDetailLabel} onViewDetail={onViewDetail} />
        </div>
      </div>

      <Button asChild size="lg" className="my-2 min-h-12 w-full sm:my-0 sm:w-auto sm:shrink-0">
        <a href={signUp.href}>
          {signUp.label}
          <span className="sr-only">{`: ${title}`}</span>
        </a>
      </Button>
    </article>
  );
}
