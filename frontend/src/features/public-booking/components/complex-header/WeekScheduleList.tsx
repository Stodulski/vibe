import { Fragment } from 'react';
import { cn } from '@/shared/lib/utils';
import type { Schedule } from '@/shared/types/api.types';
import { buildWeekSchedule } from './weekSchedule';
import { getDayName } from './dayNames';

/**
 * The venue's opening hours, all week, with today marked.
 *
 * Today is highlighted rather than shown alone. The single line this replaces
 * answered only "can I play tonight"; someone choosing a club wants to know
 * about Sunday, and someone looking on Tuesday for a Saturday match had to
 * guess. Marking today keeps that first question a glance away without
 * hiding the rest of the answer.
 *
 * One row per day, with the day stacked over its hours rather than facing
 * them across a gap. Laid out in a row, each day needed the full width of the
 * column and the eye had to travel a leader line to reach the times; stacked,
 * a day is a small two-line block, so several fit across and the times sit
 * directly under the label they belong to.
 *
 * No heading of its own: whatever sits above it is the heading.
 *
 * The type grows from 11px on a phone to 16px on a desktop for the reason
 * spelled out in `AmenityList`: the same angular size held at two viewing
 * distances. The two lists move together.
 */
interface WeekScheduleListProps {
  schedules: Schedule[];
  /**
   * The day the slot picker above is actually showing — `ComplexPageContent`
   * resolves this from `?date=` (or today, when the query has none) and
   * passes it all the way down here, so the bolded row agrees with what the
   * person is looking at instead of always bolding today.
   */
  selectedDate?: Date | undefined;
}

export function WeekScheduleList({ schedules, selectedDate }: WeekScheduleListProps) {
  const rows = buildWeekSchedule(schedules, selectedDate ? getDayName(selectedDate) : undefined);

  return (
    // Two columns while stacked; from `lg` the block spans the whole card, so
    // the seven days sit in one row and the week reads left to right, spread
    // across the full width with a rule between each pair.
    //
    // The rules are elements of their own, not borders on the cells. With
    // `justify-between` sharing the leftover width equally among thirteen
    // items — seven days and six rules — every rule lands the same distance
    // from the text on either side. A border on the cell cannot do that: in
    // equal columns the text never fills its column, so the rule sits 10px
    // from one neighbour and 20px from the other; in content-sized cells the
    // rules are equidistant but the row stops short of the width.
    //
    // The tightest point is 1024px, where the row is 742px wide: seven ranges
    // at 12px (84px each) leave 12px of air on each side of every rule, which
    // is why the range stays at 12px until `xl` (97px at 14px would leave 4).
    // Widening the type below `xl` closes those gaps, so measure again before
    // touching it.
    <dl className="grid grid-cols-2 gap-x-6 gap-y-3 lg:flex lg:justify-between lg:gap-0">
      {rows.map((row, index) => (
        <Fragment key={row.day}>
          {index > 0 && <div aria-hidden="true" className="bg-border-subtle hidden w-px self-stretch lg:block" />}
          <div>
            <dt
              className={cn(
                'text-micro tracking-wide uppercase lg:text-sm lg:tracking-normal lg:normal-case',
                row.isToday ? 'text-primary-400 font-semibold' : 'text-text-tertiary',
              )}
            >
              {row.label}
            </dt>
            <dd
              className={cn(
                // A time range is one token: "08:00 -" over "23:00" reads as
                // two different facts.
                'text-micro whitespace-nowrap tabular-nums sm:text-xs xl:text-sm',
                row.isToday ? 'text-text-primary font-semibold' : 'text-text-secondary',
              )}
            >
              {row.hours}
            </dd>
          </div>
        </Fragment>
      ))}
    </dl>
  );
}
