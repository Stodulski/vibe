import { useRef, useState } from 'react';
import { Button, cn } from '@vibe/ui';
import type { DurationMinutes } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { durationLabel, nearestStop, stopPercent } from './dialMath';

const t = BOOKING_MESSAGES.publicBooking;

interface BarProps {
  options: DurationMinutes[];
  index: number;
  onPick: (index: number) => void;
}

/** The bar itself: a track filled green up to the choice, its knob, and each duration under its stop. */
function DurationBar({ options, index, onPick }: BarProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);
  const at = stopPercent(index, options.length);
  const pick = (clientX: number) => {
    const box = trackRef.current?.getBoundingClientRect();
    if (!box || box.width === 0) return;
    onPick(nearestStop(((clientX - box.left) / box.width) * 100, options.length));
  };

  return (
    <div className="mx-auto w-full max-w-xl px-6">
      <div
        ref={trackRef}
        aria-hidden="true"
        className="relative h-12 cursor-pointer touch-none select-none"
        onPointerDown={(event) => {
          dragging.current = true;
          event.currentTarget.setPointerCapture(event.pointerId);
          pick(event.clientX);
        }}
        onPointerMove={(event) => {
          if (dragging.current) pick(event.clientX);
        }}
        onPointerUp={() => {
          dragging.current = false;
        }}
      >
        <span className="bg-border-interactive absolute top-1/2 right-0 left-0 h-2 -translate-y-1/2 rounded-full" />
        <span
          className="bg-primary-500 absolute top-1/2 left-0 h-2 -translate-y-1/2 rounded-full transition-all duration-300"
          style={{ width: `${String(at)}%` }}
        />
        <span
          className="border-primary-500 bg-bg-base absolute top-1/2 size-7 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 transition-all duration-300"
          style={{ left: `${String(at)}%` }}
        />
      </div>
      <div aria-hidden="true" className="relative h-6">
        {options.map((option, i) => (
          <span
            key={option}
            style={{ left: `${String(stopPercent(i, options.length))}%` }}
            className={cn(
              'absolute -translate-x-1/2 text-sm whitespace-nowrap',
              i === index ? 'text-primary-400 font-semibold' : 'text-text-secondary',
            )}
          >
            {durationLabel(option)}
          </span>
        ))}
      </div>
    </div>
  );
}

interface DurationDialProps {
  question: string;
  options: DurationMinutes[];
  selected: DurationMinutes | null;
  onChoose: (value: DurationMinutes) => void;
}

/**
 * How long to play, on a straight bar: the chosen duration large above it, the bar
 * filled green up to it. Drag the knob or tap a spot on the bar; the button answers.
 */
export function DurationDial({ question, options, selected, onChoose }: DurationDialProps) {
  const initial = selected === null ? -1 : options.indexOf(selected);
  // It opens on the shortest duration, 1 h, unless one was already chosen.
  const [index, setIndex] = useState(Math.max(0, initial));
  const value = options[index] ?? options[0];
  if (value === undefined) return null;

  return (
    <section aria-label={question} className="flex flex-col gap-4">
      <h2 className="text-text-primary text-center text-xl font-bold">{question}</h2>
      <p className="text-text-primary text-center text-4xl font-bold tabular-nums" aria-live="polite">
        {durationLabel(value)}
      </p>
      <DurationBar options={options} index={index} onPick={setIndex} />
      <div role="radiogroup" aria-label={question} className="sr-only">
        {options.map((option, i) => (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={i === index}
            onClick={() => {
              setIndex(i);
            }}
          >
            {durationLabel(option)}
          </button>
        ))}
      </div>
      <Button
        size="lg"
        className="min-h-12 w-full"
        onClick={() => {
          onChoose(value);
        }}
      >
        {`${t.chooseAnswer} ${durationLabel(value)}`}
      </Button>
    </section>
  );
}
