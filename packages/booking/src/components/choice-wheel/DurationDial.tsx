import { useRef, useState } from 'react';
import { Button } from '@vibe/ui';
import type { DurationMinutes } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { arcTo, CX, CY, durationLabel, pointAt, R, stopAngle } from './dialMath';

const t = BOOKING_MESSAGES.publicBooking;

/** What the dial draws: the track, the green fill up to the choice, the stops, the knob and the choice. */
function DialFace({ options, index, value }: { options: DurationMinutes[]; index: number; value: DurationMinutes }) {
  const angle = stopAngle(index, options.length);
  const knob = pointAt(angle);
  return (
    <>
    <path d={arcTo(0)} fill="none" stroke="var(--color-border-default)" strokeWidth="14" strokeLinecap="round" />
    <path d={arcTo(angle)} fill="none" stroke="var(--color-primary-500)" strokeWidth="14" strokeLinecap="round" className="transition-all duration-300" />
    {options.map((option, i) => {
      const at = pointAt(stopAngle(i, options.length), R - 34);
      return (
        <text key={option} x={at.x} y={at.y} textAnchor="middle" dominantBaseline="middle" className={i === index ? 'fill-primary-400 text-sm font-semibold' : 'fill-text-tertiary text-sm'}>
          {durationLabel(option)}
        </text>
      );
    })}
    <circle cx={knob.x} cy={knob.y} r="13" fill="var(--color-bg-base)" stroke="var(--color-primary-500)" strokeWidth="5" className="transition-all duration-300" />
    <text x={CX} y={CY - 18} textAnchor="middle" className="fill-text-primary text-4xl font-bold">
      {durationLabel(value)}
    </text>
    </>
  );
}

interface DurationDialProps {
  question: string;
  options: DurationMinutes[];
  selected: DurationMinutes | null;
  onChoose: (value: DurationMinutes) => void;
}

/**
 * How long to play, on a dial: a half circle that fills in green up to the chosen
 * duration, with the duration large in its middle. Drag the knob along the arc or tap a
 * duration; "Continuar" answers.
 */
export function DurationDial({ question, options, selected, onChoose }: DurationDialProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const dragging = useRef(false);
  const initial = selected === null ? -1 : options.indexOf(selected);
  const [index, setIndex] = useState(initial >= 0 ? initial : Math.floor((options.length - 1) / 2));
  const value = options[index] ?? options[0];
  if (value === undefined) return null;
  // The stop nearest the pointer's angle around the dial's center.
  const pick = (clientX: number, clientY: number) => {
    const svg = svgRef.current;
    if (!svg) return;
    const box = svg.getBoundingClientRect();
    const x = ((clientX - box.left) / box.width) * 300 - CX;
    const y = CY - ((clientY - box.top) / box.height) * 170;
    const pointer = Math.max(0, Math.min(180, (Math.atan2(Math.max(0, y), x) * 180) / Math.PI));
    const nearest = options.reduce((best, _, i) =>
      Math.abs(stopAngle(i, options.length) - pointer) < Math.abs(stopAngle(best, options.length) - pointer) ? i : best,
    0);
    setIndex(nearest);
  };

  return (
    <section aria-label={question} className="flex flex-col gap-4">
      <h2 className="text-text-primary text-xl font-bold">{question}</h2>
      <svg
        ref={svgRef}
        viewBox="0 0 300 170"
        className="mx-auto w-full max-w-sm cursor-pointer touch-none select-none"
        onPointerDown={(event) => {
          dragging.current = true;
          event.currentTarget.setPointerCapture(event.pointerId);
          pick(event.clientX, event.clientY);
        }}
        onPointerMove={(event) => {
          if (dragging.current) pick(event.clientX, event.clientY);
        }}
        onPointerUp={() => {
          dragging.current = false;
        }}
        aria-hidden="true"
      >
        <DialFace options={options} index={index} value={value} />
      </svg>
      <div role="radiogroup" aria-label={question} className="sr-only">
        {options.map((option, i) => (
          <button key={option} type="button" role="radio" aria-checked={i === index} onClick={() => { setIndex(i); }}>
            {durationLabel(option)}
          </button>
        ))}
      </div>
      <Button size="lg" className="min-h-12 w-full" onClick={() => { onChoose(value); }}>
        {t.rulerContinue}
      </Button>
    </section>
  );
}
