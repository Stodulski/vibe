import { Button } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { RulerHint, RulerReadout, RulerTick } from './rulerParts';
import type { TimeOption } from './timeOptions';
import { useNativeWheel, useWidth } from './useRulerBox';
import { useWheelPicker } from './useWheelPicker';
import { tickWidthFor, wrapIndex } from './wheelMath';

const t = BOOKING_MESSAGES.publicBooking;

/** The tick indices drawn: the ones on screen plus one past each edge; a single hour is not repeated. */
function visibleRange(pos: number, half: number, tick: number, count: number): number[] {
  if (count < 2) return [0];
  const reach = Math.ceil(half / tick) + 1;
  const first = Math.floor(pos) - reach;
  return Array.from({ length: Math.ceil(pos) + reach - first + 1 }, (_, i) => first + i);
}

interface TimeRulerProps {
  options: TimeOption[];
  /** The hour already chosen, centered when the ruler opens. */
  initialStartTime: string | null;
  /** Whether the venue has more than one court, so "Última cancha" means something. */
  severalCourts: boolean;
  onSelectTime: (option: TimeOption) => void;
}

/**
 * The hours of the day as an endless wheel: past the last hour comes the first again.
 * The hour at the center is shown large above it, with its end, price and free courts,
 * and "Continuar" takes it to the court question. It turns by dragging (a finger or a
 * mouse) with a fling that settles on an hour, by the mouse wheel, and by tapping an
 * hour. Every tick is drawn from the wheel's position in the same frame, so the curve
 * never lags behind a fast swipe.
 */
export function TimeRuler({ options, initialStartTime, severalCourts, onSelectTime }: TimeRulerProps) {
  const [width, element, measureRef] = useWidth();
  const tick = tickWidthFor(width);
  const half = width / 2;
  const count = options.length;
  const wheel = useWheelPicker(Math.max(0, options.findIndex((option) => option.startTime === initialStartTime)), tick);
  useNativeWheel(element, wheel.onWheel);
  const current = options[wrapIndex(wheel.center, Math.max(1, count))];
  if (!current) return null;

  // Each hour is announced once: the copy nearest the center.
  const isGhost = (index: number) => {
    const fromCenter = index - wheel.center;
    return fromCenter < -Math.floor(count / 2) || fromCenter >= Math.ceil(count / 2);
  };

  return (
    <section aria-label={t.rulerLabel} className="flex flex-col gap-4">
      {/* The readout sits in the wheel's empty top: only the ends climb that high, at the sides. */}
      <div className="relative z-10">
        <RulerReadout option={current} severalCourts={severalCourts} />
      </div>
      <div
        ref={measureRef}
        {...wheel.handlers}
        className="relative -mt-20 h-44 cursor-grab touch-pan-y overflow-hidden select-none active:cursor-grabbing"
      >
        {visibleRange(wheel.pos, half, tick, count).map((index) => {
          const option = options[wrapIndex(index, count)];
          if (!option) return null;
          return (
            <RulerTick
              key={index}
              option={option}
              offset={(index - wheel.pos) * tick}
              half={half}
              width={tick}
              focused={index === wheel.center}
              ghost={isGhost(index)}
              last={severalCourts && option.courts.length === 1}
              onPick={() => {
                wheel.goTo(index);
              }}
            />
          );
        })}
      </div>
      <RulerHint />
      <Button
        size="lg"
        className="min-h-12 w-full"
        onClick={() => {
          onSelectTime(current);
        }}
      >
        {t.rulerContinue}
      </Button>
    </section>
  );
}
