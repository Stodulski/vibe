import { useEffect, useRef, useState } from 'react';
import { Button, cn, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { applyRulerDepth } from './rulerDepth';
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

/** One hour on the ruler: a tick and its time under it. */
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
  return (
    <button
      type="button"
      aria-label={last ? `${option.startTime}, ${t.lastCourtLeft}` : option.startTime}
      data-slot-time={option.startTime}
      aria-pressed={focused}
      onClick={onSelect}
      className="flex w-14 shrink-0 cursor-pointer snap-center flex-col items-center gap-1.5 will-change-transform"
    >
      {/* The lines stand on one baseline, so every time sits at the same height. */}
      <span aria-hidden="true" className="flex h-8 items-end">
        <span
          className={cn(
            'rounded-full transition-colors',
            // Every tick is one height; only the wheel's scale makes them differ.
            focused ? 'bg-primary-500 h-8 w-1' : 'bg-border-default h-8 w-px',
          )}
        />
      </span>
      <span className={cn('text-xs tabular-nums transition-colors', focused ? 'text-primary-400 font-semibold' : 'text-text-tertiary')}>
        {option.startTime}
      </span>
    </button>
  );
}

/** Brings tick `index` to the center, smoothly. */
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

/** How to move the ruler: a finger slides it, a mouse drags it. */
function RulerHint() {
  return (
    <p className="text-text-tertiary text-center text-xs">
      <span className="pointer-fine:hidden">{t.rulerHint}</span>
      <span className="hidden pointer-fine:inline">{t.rulerHintMouse}</span>
    </p>
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
    if (track) applyRulerDepth(track);
  }, [initial]);

  if (!current) return null;

  return (
    <section aria-label={t.rulerLabel} className="flex flex-col gap-4">
      <RulerReadout option={current} severalCourts={severalCourts} />

      <div className="relative">
        <div
          ref={trackRef}
          {...dragHandlers}
          className="scrollbar-none flex cursor-grab snap-x snap-mandatory overflow-x-auto select-none active:cursor-grabbing px-[calc(50%-1.75rem)] py-2 [mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)]"
          onScroll={(event) => {
            applyRulerDepth(event.currentTarget);
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
