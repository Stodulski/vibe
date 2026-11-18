import { Button, cn, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { RulerHour } from './rulerHours';
import type { TimeOption } from './timeOptions';
import { tickDepth } from './wheelMath';

const t = BOOKING_MESSAGES.publicBooking;

/** What one hour costs and how many courts it has: "$32.000 · 2 canchas", "desde $28.000 · Última cancha". */
function priceLine(option: TimeOption, severalCourts: boolean): string {
  const price = formatPrice(option.minPrice);
  const courts =
    severalCourts && option.courts.length === 1
      ? t.lastCourtLeft
      : option.courts.length === 1
        ? t.courtsFreeOne
        : `${String(option.courts.length)} ${t.courtsFreeMany}`;
  return `${option.priceVaries ? `${t.priceFrom} ` : ''}${price} · ${courts}`;
}

interface RulerTickProps {
  hour: RulerHour;
  /** Pixels from the wheel's center to this tick's center. */
  offset: number;
  half: number;
  width: number;
  focused: boolean;
  last: boolean;
  onPick: () => void;
}

/** One hour on the wheel: a tick and its time under it, placed and sized by its offset. A taken hour is amber. */
export function RulerTick({ hour, offset, half, width, focused, last, onPick }: RulerTickProps) {
  const depth = tickDepth(offset, half, width);
  const taken = hour.free === null;
  const label = taken ? `${hour.startTime}, ${t.rulerTaken}` : last ? `${hour.startTime}, ${t.lastCourtLeft}` : hour.startTime;
  return (
    <button
      type="button"
      aria-label={label}
      data-slot-time={hour.startTime}
      data-taken={taken || undefined}
      aria-pressed={focused}
      onClick={onPick}
      style={{
        width,
        // Placed from the real center (50%), so a stale width can never shift the wheel sideways.
        left: `calc(50% + ${(offset - width / 2).toFixed(1)}px)`,
        opacity: depth.opacity,
        transform: `translateY(${depth.rise.toFixed(1)}px) scale(${depth.scale.toFixed(3)})`,
      }}
      className="absolute bottom-2 flex origin-bottom cursor-pointer flex-col items-center gap-1.5 sm:gap-2"
    >
      <span aria-hidden="true" className="flex h-12 items-end">
        <span
          className={cn(
            'rounded-full',
            // Every tick is one height; only the wheel's scale makes them differ.
            focused ? 'h-12 w-1.5' : 'h-12 w-0.5',
            taken ? 'bg-warning-text' : focused ? 'bg-primary-500' : 'bg-text-tertiary',
          )}
        />
      </span>
      <span
        className={cn(
          'text-xs tabular-nums sm:text-base',
          taken ? 'text-warning-text' : focused ? 'text-primary-400' : 'text-text-secondary',
          focused && 'font-semibold',
        )}
      >
        {hour.startTime}
      </span>
    </button>
  );
}

/** The hour at the center, large, with its end and its price and free courts, or "Ocupado" in amber. */
export function RulerReadout({ hour, severalCourts }: { hour: RulerHour; severalCourts: boolean }) {
  return (
    <div className="text-center" aria-live="polite">
      <p className={cn('text-3xl font-bold tabular-nums', hour.free ? 'text-text-primary' : 'text-warning-text')}>
        {hour.startTime} <span className="text-text-tertiary">–</span> {hour.endTime}
      </p>
      <p className={cn('mt-1 text-sm', hour.free ? 'text-text-secondary' : 'text-warning-text')}>
        {hour.free ? priceLine(hour.free, severalCourts) : t.rulerTaken}
      </p>
    </div>
  );
}

/** How to move the ruler: a finger slides it, a mouse drags it. */
export function RulerHint() {
  return (
    <p className="text-text-tertiary text-center text-xs">
      <span className="pointer-fine:hidden">{t.rulerHint}</span>
      <span className="hidden pointer-fine:inline">{t.rulerHintMouse}</span>
    </p>
  );
}

interface RulerActionProps {
  hour: RulerHour;
  onSelectTime: (option: TimeOption) => void;
  /** Turns the wheel to the nearest free hour; `null` when the day has none. */
  onGoToFree: (() => void) | null;
}

/** A free hour opens its courts; a taken one offers the nearest free hour instead. */
export function RulerAction({ hour, onSelectTime, onGoToFree }: RulerActionProps) {
  const free = hour.free;
  if (free) {
    return (
      <Button
        size="lg"
        className="min-h-12 w-full"
        onClick={() => {
          onSelectTime(free);
        }}
      >
        {`${t.seeCourtsAt} ${hour.startTime}`}
      </Button>
    );
  }
  if (!onGoToFree) return null;
  return (
    <Button variant="outline" size="lg" className="min-h-12 w-full" onClick={onGoToFree}>
      {t.goToNearestFree}
    </Button>
  );
}
