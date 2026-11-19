import { BOOKING_MESSAGES } from '../../messages';
import { nearestFree, type RulerHour } from './rulerHours';
import { RulerAction, RulerHint, RulerReadout, RulerTick } from './rulerParts';
import type { TimeOption } from './timeOptions';
import { useNativeWheel, useWidth } from './useRulerBox';
import { useWheelPicker } from './useWheelPicker';
import { tickWidthFor } from './wheelMath';

const t = BOOKING_MESSAGES.publicBooking;

/** The tick indices drawn: the hours on screen plus one past each edge, within the day. */
function visibleRange(pos: number, half: number, tick: number, count: number): number[] {
  const reach = Math.ceil(half / tick) + 1;
  const first = Math.max(0, Math.floor(pos) - reach);
  const last = Math.min(count - 1, Math.ceil(pos) + reach);
  return Array.from({ length: Math.max(0, last - first + 1) }, (_, i) => first + i);
}

interface TimeRulerProps {
  /** Every hour of the day, free or taken (see `buildRulerHours`). */
  hours: RulerHour[];
  /** The hour already chosen, centered when the ruler opens. */
  initialStartTime: string | null;
  /** Whether the venue has more than one court, so "Última cancha" means something. */
  severalCourts: boolean;
  onSelectTime: (option: TimeOption) => void;
  /** The complex and day a taken hour's "Avisarme cuando se libere" waits on. */
  waitlist?: { slug: string; date: string } | null;
}

/**
 * The hours of the day as a wheel, from the first to the last; it opens on the middle of
 * the day unless an hour was already chosen. The hour at the center is shown large above it, with its end, price and free courts,
 * and "Continuar" takes it to the court question. It turns by dragging (a finger or a
 * mouse) with a fling that settles on an hour, by the mouse wheel, and by tapping an
 * hour. Every tick is drawn from the wheel's position in the same frame, so the curve
 * never lags behind a fast swipe.
 */
export function TimeRuler({ hours, initialStartTime, severalCourts, onSelectTime, waitlist = null }: TimeRulerProps) {
  const [width, element, measureRef] = useWidth();
  const tick = tickWidthFor(width);
  const half = width / 2;
  const count = hours.length;
  // It opens on the hour already chosen, or else on the free hour nearest the middle of the day.
  const chosen = hours.findIndex((hour) => hour.startTime === initialStartTime);
  const middle = Math.floor((count - 1) / 2);
  const wheel = useWheelPicker(chosen >= 0 ? chosen : (nearestFree(hours, middle) ?? middle), count, tick);
  useNativeWheel(element, wheel.onWheel);
  const current = hours[Math.min(count - 1, Math.max(0, wheel.center))];
  if (!current) return null;
  const toFree = current.free ? null : nearestFree(hours, wheel.center);

  return (
    <section aria-label={t.rulerLabel} className="flex flex-col gap-4">
      <h2 className="text-text-primary text-center text-xl font-bold">{t.timeQuestion}</h2>
      {/* The readout sits in the wheel's empty top: only the ends climb that high, at the sides. */}
      <div className="relative z-10">
        <RulerReadout hour={current} severalCourts={severalCourts} />
      </div>
      <div
        ref={measureRef}
        {...wheel.handlers}
        className="relative -mt-20 h-44 cursor-grab touch-pan-y overflow-hidden select-none active:cursor-grabbing"
      >
        {visibleRange(wheel.pos, half, tick, count).map((index) => {
          const hour = hours[index];
          if (!hour) return null;
          return (
            <RulerTick
              key={index}
              hour={hour}
              offset={(index - wheel.pos) * tick}
              half={half}
              width={tick}
              focused={index === wheel.center}
              last={severalCourts && hour.free?.courts.length === 1}
              onPick={() => {
                wheel.goTo(index);
              }}
            />
          );
        })}
      </div>
      <RulerHint />
      <RulerAction
        hour={current}
        onSelectTime={onSelectTime}
        onGoToFree={toFree === null ? null : () => { wheel.goTo(toFree); }}
        waitlist={waitlist}
      />
    </section>
  );
}
