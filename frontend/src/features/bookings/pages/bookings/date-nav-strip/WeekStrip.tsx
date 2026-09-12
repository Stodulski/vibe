import { format } from 'date-fns/format';
import { addDays } from 'date-fns/addDays';
import { subDays } from 'date-fns/subDays';
import { parseISO } from 'date-fns/parseISO';
import { isToday as isTodayFn } from 'date-fns/isToday';
import { es } from 'date-fns/locale/es';
import { cn } from '@/shared/lib/utils';
import { useElementWidth } from '@/shared/hooks/useElementWidth';

// How much room one day gets. Sized well above what "MIÉ 26" strictly needs:
// the count follows the available width, and once the strip lost its card it
// had the whole page to fill — three weeks of tiny tiles to pick one day from.
const MIN_DAY_WIDTH = 96;
const MIN_DAYS = 5;
// Two weeks is already more than anyone scans; past that the strip stops
// being a picker and becomes a wall.
const MAX_DAYS = 15;
// jsdom never lays out, so a measured width of 0 means "not measured yet"
// rather than "no room"; a week is the sensible thing to render meanwhile.
const FALLBACK_DAYS = 7;

function visibleDayCount(width: number): number {
  if (width === 0) return FALLBACK_DAYS;
  const fits = Math.floor(width / MIN_DAY_WIDTH);
  const bounded = Math.min(Math.max(fits, MIN_DAYS), MAX_DAYS);
  // Odd, so the selected day sits exactly in the middle.
  return bounded % 2 === 0 ? bounded - 1 : bounded;
}

export function WeekStrip({
  selectedDate,
  onDateSelect,
}: {
  selectedDate: string;
  onDateSelect: (date: string) => void;
}) {
  const { ref, width } = useElementWidth();

  const count = visibleDayCount(width);
  const start = subDays(parseISO(selectedDate), (count - 1) / 2);
  const days = Array.from({ length: count }, (_, i) => addDays(start, i));

  return (
    <div ref={ref} className="flex items-stretch gap-0.5 sm:gap-1">
      {days.map((day) => {
        const dayStr = format(day, 'yyyy-MM-dd');
        const isSel = dayStr === selectedDate;
        const isDayToday = isTodayFn(day);
        return (
          <button
            key={dayStr}
            onClick={() => {
              onDateSelect(dayStr);
            }}
            aria-pressed={isSel}
            className={cn(
              'relative flex min-h-0 flex-1 flex-col items-center rounded-xl py-2 text-xs transition-colors duration-150 sm:py-2.5',
              isSel
                ? 'bg-primary-500/10 text-primary-400 shadow-sm shadow-primary-500/5'
                : isDayToday
                  ? 'text-text-primary hover:bg-bg-elevated/50'
                  : 'text-text-tertiary hover:bg-bg-elevated/50 hover:text-text-secondary',
            )}
          >
            <span className="whitespace-nowrap text-xs font-medium uppercase tracking-wider">
              {format(day, 'EEE', { locale: es })}
            </span>
            <span className={cn('mt-0.5 whitespace-nowrap text-base font-bold', isSel && 'text-primary-300')}>
              {format(day, 'd')}
            </span>
            {isDayToday && (
              <div
                className={cn('absolute bottom-1 size-1 rounded-full', isSel ? 'bg-primary-400' : 'bg-primary-500')}
              />
            )}
          </button>
        );
      })}
    </div>
  );
}
