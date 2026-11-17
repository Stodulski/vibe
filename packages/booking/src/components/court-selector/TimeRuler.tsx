import { useEffect, useRef, useState } from 'react';
import { Button, cn, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { applyRulerDepth } from './rulerDepth';
import { LOOP_COPIES, useLoopWrap } from './rulerLoop';
import { centeredIndex, centerTick, useMouseDrag, useWheelSlides } from './rulerTrack';
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

/** One hour on the ruler: a tick and its time under it. */
function RulerTick({
  option,
  focused,
  ghost,
  last,
  onSelect,
}: {
  option: TimeOption;
  focused: boolean;
  /** A tick of the loop's outer copies: drawn and tappable, but not announced twice. */
  ghost: boolean;
  last: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={last ? `${option.startTime}, ${t.lastCourtLeft}` : option.startTime}
      data-slot-time={ghost ? undefined : option.startTime}
      aria-hidden={ghost || undefined}
      tabIndex={ghost ? -1 : undefined}
      aria-pressed={focused}
      onClick={onSelect}
      className="flex w-12 shrink-0 cursor-pointer snap-center flex-col items-center gap-1.5 will-change-transform sm:w-20 sm:gap-2"
    >
      {/* The lines stand on one baseline, so every time sits at the same height. */}
      <span aria-hidden="true" className="flex h-12 items-end">
        <span
          className={cn(
            'rounded-full transition-colors',
            // Every tick is one height; only the wheel's scale makes them differ.
            focused ? 'bg-primary-500 h-12 w-1.5' : 'bg-border-default h-12 w-0.5',
          )}
        />
      </span>
      <span className={cn('text-xs tabular-nums transition-colors sm:text-base', focused ? 'text-primary-400 font-semibold' : 'text-text-tertiary')}>
        {option.startTime}
      </span>
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

/** How to move the ruler: a finger slides it, a mouse drags it. */
function RulerHint() {
  return (
    <p className="text-text-tertiary text-center text-xs">
      <span className="pointer-fine:hidden">{t.rulerHint}</span>
      <span className="hidden pointer-fine:inline">{t.rulerHintMouse}</span>
    </p>
  );
}

/** Every tick of the endless ruler: the hours `copies` times over, only the middle copy announced. */
function RulerTicks({
  options,
  copies,
  focused,
  severalCourts,
  onPick,
}: {
  options: TimeOption[];
  copies: number;
  focused: number;
  severalCourts: boolean;
  onPick: (index: number) => void;
}) {
  const count = options.length;
  return Array.from({ length: count * copies }, (_, index) => {
    const option = options[index % count];
    if (!option) return null;
    return (
      <RulerTick
        key={`${String(Math.floor(index / count))}-${option.startTime}`}
        option={option}
        focused={index === focused}
        ghost={copies > 1 && Math.floor(index / count) !== 1}
        last={isLastCourt(option, severalCourts)}
        onSelect={() => {
          onPick(index);
        }}
      />
    );
  });
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
 * finger or a trackpad, by dragging it with the mouse, and with the mouse wheel, and
 * it is endless: past the last hour comes the first again.
 */
export function TimeRuler({ options, initialStartTime, severalCourts, onSelectTime }: TimeRulerProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const count = options.length;
  // An endless ruler: the hours three times over, starting in the middle copy.
  const copies = count > 1 ? LOOP_COPIES : 1;
  const middle = copies > 1 ? count : 0;
  const initial = middle + Math.max(0, options.findIndex((option) => option.startTime === initialStartTime));
  const [focused, setFocused] = useState(initial);
  useWheelSlides(trackRef);
  const dragHandlers = useMouseDrag(trackRef);
  const wrap = useLoopWrap(count);
  const current = options[focused % Math.max(1, count)];

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
          className="scrollbar-none flex cursor-grab snap-x snap-mandatory overflow-x-auto select-none active:cursor-grabbing px-[calc(50%-1.5rem)] pt-24 sm:px-[calc(50%-2.5rem)] pb-2 [mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)]"
          onScroll={(event) => {
            applyRulerDepth(event.currentTarget);
            setFocused(centeredIndex(event.currentTarget));
            wrap(event.currentTarget);
          }}
        >
          <RulerTicks
            options={options}
            copies={copies}
            focused={focused}
            severalCourts={severalCourts}
            onPick={(index) => {
              setFocused(index);
              centerTick(trackRef.current, index);
            }}
          />
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
