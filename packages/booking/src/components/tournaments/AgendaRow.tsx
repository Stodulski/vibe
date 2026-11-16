import { Fragment } from 'react';
import { Button, cn, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentPath } from '../../lib/publicPaths';
import { dayParts, spotsNotice, type AgendaRow as AgendaRowData } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

interface AgendaRowProps {
  row: AgendaRowData;
  slug: string;
  onViewDetail: (row: AgendaRowData) => void;
}

/**
 * The left of a row: the day as the agenda writes it (the weekday small, the
 * day number large in the accent, the month small), with a thin accent rule
 * after it.
 */
function DateBlock({ date }: { date: string }) {
  const parts = dayParts(date);
  return (
    <div className="flex shrink-0 items-stretch gap-4">
      <time dateTime={date} className="flex w-14 flex-col items-center justify-center text-center">
        <span className="text-text-secondary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">
          {parts.weekday}
        </span>
        <span className="text-primary-400 text-3xl leading-none font-bold tabular-nums">{parts.day}</span>
        <span className="text-text-secondary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">
          {parts.month}
        </span>
      </time>
      <span aria-hidden="true" className="bg-primary-500 w-px rounded-full" />
    </div>
  );
}

/**
 * One category on the agenda: its day and time, its title under the sport's
 * label, one line of what it costs and how many places are left, and its
 * sign-up on the right on a desktop and under the line on a phone. Each token
 * of the line keeps its words together, so a phone breaks the line between
 * tokens only.
 */
export function AgendaRow({ row, slug, onViewDetail }: AgendaRowProps) {
  const { category } = row;
  const spots = spotsNotice(category.spots_left);
  const price = `${formatPrice(category.price)} ${t.perPair}`;
  const meta: { key: string; text: string; emphasis?: boolean }[] = [
    ...(row.time === null ? [] : [{ key: 'time', text: `${row.time} ${t.hoursSuffix}` }]),
    { key: 'spots', text: spots.text, ...(spots.last ? { emphasis: true } : {}) },
    { key: 'price', text: price },
  ];

  return (
    <article className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:gap-6">
      <div className="flex min-w-0 flex-1 items-center gap-5">
        <DateBlock date={row.date} />
        <div className="flex min-w-0 flex-col gap-1">
          <p className="text-text-tertiary text-xs font-semibold tracking-wider whitespace-nowrap uppercase">
            {row.label}
          </p>
          <h3 className="text-text-primary text-lg leading-tight font-bold">{row.title}</h3>
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
              onClick={() => {
                onViewDetail(row);
              }}
              className="text-primary-400 hover:text-primary-300 cursor-pointer font-medium whitespace-nowrap underline-offset-4 hover:underline"
            >
              {t.viewDetail}
            </button>
          </p>
        </div>
      </div>

      <Button asChild size="lg" className="min-h-12 w-full sm:w-auto sm:shrink-0">
        <a href={publicTournamentPath(slug, row.tournamentId, category.id)}>
          {t.signUpCategory}
          <span className="sr-only">{`: ${row.title}`}</span>
        </a>
      </Button>
    </article>
  );
}
