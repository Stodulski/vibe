import { useEffect, useRef, useState } from 'react';
import { Button, cn, formatPrice } from '@vibe/ui';
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

/**
 * Dragging the ruler with a mouse, as a finger does on a phone. Snapping is off while
 * it moves, and on release the nearest hour settles under the marker. A drag never
 * counts as a tap on the tick it ends over.
 */
function useMouseDrag(trackRef: React.RefObject<HTMLDivElement | null>) {
  const drag = useRef<{ x: number; left: number; moved: boolean } | null>(null);
  const justDragged = useRef(false);
  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.pointerType !== 'mouse' || event.button !== 0) return;
    drag.current = { x: event.clientX, left: event.currentTarget.scrollLeft, moved: false };
  };
  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const state = drag.current;
    if (!state) return;
    const dx = event.clientX - state.x;
    if (!state.moved && Math.abs(dx) < 5) return;
    if (!state.moved) {
      state.moved = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      event.currentTarget.style.scrollSnapType = 'none';
    }
    event.currentTarget.scrollLeft = state.left - dx;
  };
  const onPointerEnd = (event: React.PointerEvent<HTMLDivElement>) => {
    const state = drag.current;
    drag.current = null;
    if (!state?.moved) return;
    justDragged.current = true;
    const track = event.currentTarget;
    track.style.scrollSnapType = '';
    centerTick(trackRef.current, centeredIndex(track));
  };
  // Swallows the click a drag ends with, so the tick under the pointer is not tapped.
  const onClickCapture = (event: React.MouseEvent) => {
    if (!justDragged.current) return;
    justDragged.current = false;
    event.stopPropagation();
    event.preventDefault();
  };
  return { onPointerDown, onPointerMove, onPointerUp: onPointerEnd, onPointerCancel: onPointerEnd, onClickCapture };
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
 * finger or a trackpad, by dragging it with the mouse, and with the mouse wheel.
 */
export function TimeRuler({ options, initialStartTime, severalCourts, onSelectTime }: TimeRulerProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const initial = Math.max(
    0,
    options.findIndex((option) => option.startTime === initialStartTime),
  );
  const [focused, setFocused] = useState(initial);
  useWheelSlides(trackRef);
  const dragHandlers = useMouseDrag(trackRef);
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
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- the drag is a mouse shortcut; the ticks and Continuar are the accessible controls */}
        <div
          ref={trackRef}
          {...dragHandlers}
          className="scrollbar-none flex cursor-grab snap-x snap-mandatory overflow-x-auto select-none active:cursor-grabbing px-[calc(50%-1.75rem)] py-2 [mask-image:linear-gradient(to_right,transparent,black_25%,black_75%,transparent)]"
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
