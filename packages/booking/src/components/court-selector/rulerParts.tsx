import { cn, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
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
  option: TimeOption;
  /** Pixels from the wheel's center to this tick's center. */
  offset: number;
  half: number;
  width: number;
  focused: boolean;
  last: boolean;
  onPick: () => void;
}

/** One hour on the wheel: a tick and its time under it, placed and sized by its offset. */
export function RulerTick({ option, offset, half, width, focused, last, onPick }: RulerTickProps) {
  const depth = tickDepth(offset, half, width);
  return (
    <button
      type="button"
      aria-label={last ? `${option.startTime}, ${t.lastCourtLeft}` : option.startTime}
      data-slot-time={option.startTime}
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
            focused ? 'bg-primary-500 h-12 w-1.5' : 'bg-border-default h-12 w-0.5',
          )}
        />
      </span>
      <span
        className={cn(
          'text-xs tabular-nums sm:text-base',
          focused ? 'text-primary-400 font-semibold' : 'text-text-tertiary',
        )}
      >
        {option.startTime}
      </span>
    </button>
  );
}

/** The hour at the center, large, with its end, price and free courts. */
export function RulerReadout({ option, severalCourts }: { option: TimeOption; severalCourts: boolean }) {
  return (
    <div className="text-center" aria-live="polite">
      <p className="text-text-primary text-3xl font-bold tabular-nums">
        {option.startTime} <span className="text-text-tertiary">–</span> {option.endTime}
      </p>
      <p className="text-text-secondary mt-1 text-sm">{priceLine(option, severalCourts)}</p>
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
