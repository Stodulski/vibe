import { Check } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const STEPS = [t.publicBooking.stepSelect, t.publicBooking.stepData, t.publicBooking.stepPay] as const;

interface StepIndicatorProps {
  currentStep: 1 | 2 | 3;
}

export function StepIndicator({ currentStep }: StepIndicatorProps) {
  // On mobile every step's label is hidden except the active one's (below,
  // as its own line) — the circles alone answer "how many steps", not "which
  // one am I on now", and that is the one a phone screen has room to keep.
  const activeLabel = STEPS[currentStep - 1];

  return (
    <div className="flex flex-col items-center gap-1.5">
      <nav aria-label={t.publicBooking.stepProgress} className="flex items-center justify-center gap-0">
        {STEPS.map((label, i) => {
          const step = i + 1;
          const isActive = step === currentStep;
          const isCompleted = step < currentStep;

          return (
            <div key={label} className="flex items-center" aria-current={isActive ? 'step' : undefined}>
              {i > 0 && (
                <div
                  className={cn(
                    'h-px w-8 transition-colors duration-300 sm:w-12',
                    isCompleted ? 'bg-primary-500' : 'bg-border-subtle',
                  )}
                />
              )}
              <div className="flex items-center gap-2">
                <div
                  className={cn(
                    'flex size-7 items-center justify-center rounded-full text-xs font-bold transition-colors duration-300',
                    isActive
                      ? 'bg-primary-500 text-bg-base shadow-brand'
                      : isCompleted
                        ? 'bg-primary-500/20 text-primary-400'
                        : 'bg-bg-elevated text-text-tertiary',
                  )}
                >
                  {isCompleted ? <Check className="size-3.5" /> : step}
                </div>
                <span
                  className={cn(
                    'hidden text-xs font-semibold sm:inline',
                    isActive ? 'text-text-primary' : isCompleted ? 'text-primary-400' : 'text-text-tertiary',
                  )}
                >
                  {label}
                </span>
              </div>
            </div>
          );
        })}
      </nav>
      <p className="text-xs font-semibold text-text-primary sm:hidden">
        {t.publicBooking.stepOfPrefix} {currentStep} {t.publicBooking.stepOfMiddle} {STEPS.length}: {activeLabel}
      </p>
    </div>
  );
}
