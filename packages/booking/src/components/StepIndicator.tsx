import { Check } from 'lucide-react';
import { cn } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../messages';
import { BOOKING_STEPS } from './checkoutSteps';

const t = BOOKING_MESSAGES;

/**
 * `default` sits on the page. `brand` sits on the checkout's green band, where
 * the page's own greens would vanish, so it is drawn in the band's ink instead.
 */
export type StepIndicatorTone = 'default' | 'brand';

interface StepIndicatorProps {
  /** The step on screen, counted from 1. */
  currentStep: number;
  /** The labels of the flow's steps, in order. Defaults to a booking's. */
  steps?: readonly string[];
  tone?: StepIndicatorTone;
}

type StepState = 'active' | 'done' | 'todo';

/**
 * Each tone's colours: the circle, the line before it, and the label, by where the step sits.
 *
 * The brand tone is drawn in full-strength ink with no opacity, so it holds the
 * contrast the green band needs: the ink on `primary-500` is about 7.6:1 (text
 * needs 4.5:1, shapes 3:1). Completed steps are solid, the active one is solid
 * with a ring to stand apart, upcoming ones are outlined, and the connectors
 * turn dashed once they lie ahead.
 */
const TONE_CLASSES = {
  default: {
    circle: {
      active: 'bg-primary-500 text-bg-base shadow-brand',
      done: 'bg-primary-500/20 text-primary-400',
      todo: 'bg-bg-elevated text-text-tertiary',
    },
    line: { done: 'h-px bg-primary-500', todo: 'h-px bg-border-subtle' },
    label: { active: 'text-text-primary', done: 'text-primary-400', todo: 'text-text-tertiary' },
  },
  brand: {
    circle: {
      active: 'bg-text-inverse text-primary-500 ring-text-inverse ring-offset-primary-500 ring-2 ring-offset-2',
      done: 'bg-text-inverse text-primary-500',
      todo: 'border-text-inverse text-text-inverse border-2',
    },
    line: { done: 'h-0.5 bg-text-inverse', todo: 'h-0 border-t-2 border-dashed border-text-inverse' },
    label: { active: 'text-text-inverse', done: 'text-text-inverse', todo: 'text-text-inverse' },
  },
} as const;

export function StepIndicator({ currentStep, steps = BOOKING_STEPS, tone = 'default' }: StepIndicatorProps) {
  // On mobile every step's label is hidden except the active one's (below,
  // as its own line) — the circles alone answer "how many steps", not "which
  // one am I on now", and that is the one a phone screen has room to keep.
  const activeLabel = steps[currentStep - 1];
  const classes = TONE_CLASSES[tone];

  return (
    <div className="flex flex-col items-center gap-1.5">
      <nav aria-label={t.publicBooking.stepProgress} className="flex items-center justify-center gap-0">
        {steps.map((label, i) => {
          const step = i + 1;
          const isActive = step === currentStep;
          const isCompleted = step < currentStep;
          const state: StepState = isActive ? 'active' : isCompleted ? 'done' : 'todo';

          return (
            <div key={label} className="flex items-center" aria-current={isActive ? 'step' : undefined}>
              {i > 0 && (
                <div
                  className={cn(
                    'mx-2 w-8 transition-colors duration-300 sm:w-12',
                    isCompleted ? classes.line.done : classes.line.todo,
                  )}
                />
              )}
              <div className="flex items-center gap-2">
                <div
                  className={cn(
                    'flex size-7 items-center justify-center rounded-full text-xs font-bold transition-colors duration-300',
                    classes.circle[state],
                  )}
                >
                  {isCompleted ? <Check className="size-3.5" /> : step}
                </div>
                <span className={cn('hidden text-xs font-semibold sm:inline', classes.label[state])}>{label}</span>
              </div>
            </div>
          );
        })}
      </nav>
      <p className={cn('text-xs font-semibold sm:hidden', classes.label.active)}>
        {t.publicBooking.stepOfPrefix} {currentStep} {t.publicBooking.stepOfMiddle} {steps.length}: {activeLabel}
      </p>
    </div>
  );
}
