import { useEffect, useRef, useState } from 'react';
import { Button, cn, formatPrice } from '@vibe/ui';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { BOOKING_MESSAGES } from '../../messages';
import type { TimeOption } from './timeOptions';

const t = BOOKING_MESSAGES.publicBooking;

/** Whether the hour has one court left at a venue that has several: "Última cancha". */
function isLastCourt(option: TimeOption, severalCourts: boolean): boolean {
  return severalCourts && option.courts.length === 1;
}

/** What one hour costs and how many courts it has: "$32.000 · 2 canchas", "desde $28.000 · Última cancha". */
function priceLine(option: TimeOption, severalCourts: boolean): string {
  const price = formatPrice(option.minPrice);
  const courts = isLastCourt(option, severalCourts)
    ? t.lastCourtLeft
    : option.courts.length === 1
      ? t.courtsFreeOne
      : `${String(option.courts.length)} ${t.courtsFreeMany}`;
  return `${option.priceVaries ? `${t.priceFrom} ` : ''}${price} · ${courts}`;
}

/** The index of the tick whose center is closest to the track's center. */
function centeredIndex(track: HTMLElement): number {
  const middle = track.scrollLeft + track.clientWidth / 2;
  let best = 0;
  let distance = Number.POSITIVE_INFINITY;
  Array.from(track.children).forEach((child, index) => {
    const el = child as HTMLElement;
    const d = Math.abs(el.offsetLeft + el.offsetWidth / 2 - middle);
    if (d < distance) {
      distance = d;
      best = index;
    }
  });
  return best;
}

/** One hour on the ruler: a tick, taller on the hour, and its time under it. */
function RulerTick({
  option,
  focused,
  last,
  onSelect,
}: {
  option: TimeOption;
  focused: boolean;
  last: boolean;
  onSelect: () => void;
}) {
  const onHour = option.startTime.endsWith(':00');
  return (
    <button
      type="button"
      aria-label={last ? `${option.startTime}, ${t.lastCourtLeft}` : option.startTime}
      data-slot-time={option.startTime}
      aria-pressed={focused}
      onClick={onSelect}
      className="flex w-14 shrink-0 cursor-pointer snap-center flex-col items-center gap-1.5"
    >
      <span
        aria-hidden="true"
        className={cn('w-px rounded-full transition-colors', onHour ? 'h-8' : 'h-5', focused ? 'bg-primary-400' : 'bg-border-default')}
      />
      <span className={cn('text-xs tabular-nums transition-colors', focused ? 'text-primary-400 font-semibold' : 'text-text-tertiary')}>
        {option.startTime}
      </span>
    </button>
  );
}

/** Brings tick `index` under the marker, smoothly. */
function centerTick(track: HTMLElement | null, index: number) {
  const tick = track?.children[index] as HTMLElement | undefined;
  if (!track || !tick) return;
  track.scrollTo({ left: tick.offsetLeft + tick.offsetWidth / 2 - track.clientWidth / 2, behavior: 'smooth' });
}

/** A mouse wheel moves the ruler sideways: a desktop without a trackpad has no other way to slide it. */
function useWheelSlides(trackRef: React.RefObject<HTMLDivElement | null>) {
  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    const onWheel = (event: WheelEvent) => {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      event.preventDefault();
      track.scrollBy({ left: event.deltaY });
    };
    track.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      track.removeEventListener('wheel', onWheel);
    };
  }, [trackRef]);
}

/** The ‹ › buttons at the ruler's ends: one hour back or forward. */
function RulerArrow({ side, disabled, onClick }: { side: 'prev' | 'next'; disabled: boolean; onClick: () => void }) {
  const Icon = side === 'prev' ? ChevronLeft : ChevronRight;
  return (
    <button
      type="button"
      aria-label={side === 'prev' ? t.rulerPrev : t.rulerNext}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'border-border-subtle bg-bg-subtle text-text-primary hover:bg-bg-overlay absolute top-3 z-20 flex size-9 cursor-pointer items-center justify-center rounded-full border transition-opacity disabled:cursor-default disabled:opacity-0',
        side === 'prev' ? 'left-0' : 'right-0',
      )}
    >
      <Icon className="size-4" aria-hidden="true" />
    </button>
  );
}

/** The hour on the marker, large, with its end, price and free courts. */
function RulerReadout({ option, severalCourts }: { option: TimeOption; severalCourts: boolean }) {
  return (
    <div className="text-center" aria-live="polite">
      <p className="text-text-primary text-3xl font-bold tabular-nums">
        {option.startTime} <span className="text-text-tertiary">–</span> {option.endTime}
      </p>
      <p className="text-text-secondary mt-1 text-sm">{priceLine(option, severalCourts)}</p>
    </div>
  );
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
 * The hours of the day as a ruler that slides under a fixed marker. The hour on
 * the marker is shown large above it, with its end, price and free courts, and
 * "Continuar" takes it to the court question. Tapping a tick brings it under the marker. It slides with a
 * finger or a trackpad, with the mouse wheel, and with the ‹ › buttons at its ends.
 */
export function TimeRuler({ options, initialStartTime, severalCourts, onSelectTime }: TimeRulerProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const initial = Math.max(
    0,
    options.findIndex((option) => option.startTime === initialStartTime),
  );
  const [focused, setFocused] = useState(initial);
  useWheelSlides(trackRef);
  const current = options[Math.min(focused, options.length - 1)];

  // Centers the starting hour once, without animation.
  useEffect(() => {
    const track = trackRef.current;
    const tick = track?.children[initial] as HTMLElement | undefined;
    if (track && tick) track.scrollLeft = tick.offsetLeft + tick.offsetWidth / 2 - track.clientWidth / 2;
  }, [initial]);

  if (!current) return null;

  return (
    <section aria-label={t.rulerLabel} className="flex flex-col gap-4">
      <RulerReadout option={current} severalCourts={severalCourts} />

      <div className="relative">
        {/* The fixed marker the ruler slides under. */}
        <span aria-hidden="true" className="bg-primary-500 pointer-events-none absolute top-2 left-1/2 z-10 h-8 w-0.5 -translate-x-1/2 rounded-full" />
        <RulerArrow side="prev" disabled={focused === 0} onClick={() => { centerTick(trackRef.current, focused - 1); }} />
        <RulerArrow side="next" disabled={focused >= options.length - 1} onClick={() => { centerTick(trackRef.current, focused + 1); }} />
        <div
          ref={trackRef}
          className="scrollbar-none flex snap-x snap-mandatory overflow-x-auto px-[calc(50%-1.75rem)] py-2 [mask-image:linear-gradient(to_right,transparent,black_25%,black_75%,transparent)]"
          onScroll={(event) => {
            setFocused(centeredIndex(event.currentTarget));
          }}
        >
          {options.map((option, index) => (
            <RulerTick
              key={option.startTime}
              option={option}
              focused={index === focused}
              last={isLastCourt(option, severalCourts)}
              onSelect={() => {
                setFocused(index);
                centerTick(trackRef.current, index);
              }}
            />
          ))}
        </div>
      </div>
      <p className="text-text-tertiary text-center text-xs">{t.rulerHint}</p>

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
