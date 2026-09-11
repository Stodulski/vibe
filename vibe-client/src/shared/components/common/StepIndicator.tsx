import { Check } from 'lucide-react';
import { cn } from '@/shared/lib/utils';

interface StepIndicatorProps {
  currentStep: number;
  totalSteps: number;
  /** Screen-reader-only label for each step, in order (index 0 = step 1). */
  stepLabels: string[];
  ariaLabel: string;
}

/**
 * Numbered step progress dots connected by a line — generalised from the
 * register form's step indicator so any multi-step form (register, the
 * create-booking modal) can share one implementation. Step names are
 * announced to screen readers but not shown visually; only the number/check
 * is rendered, matching the original register-form design.
 */
export function StepIndicator({ currentStep, totalSteps, stepLabels, ariaLabel }: StepIndicatorProps) {
  const steps = Array.from({ length: totalSteps }, (_, i) => i + 1);

  return (
    <nav aria-label={ariaLabel}>
      <ol className="flex items-center justify-center gap-0">
        {steps.map((step) => {
          const isActive = step === currentStep;
          const isCompleted = step < currentStep;
          const label = stepLabels[step - 1];

          return (
            <li key={step} className="flex items-center" aria-current={isActive ? 'step' : undefined}>
              {step > 1 && (
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
                {label && <span className="sr-only">{label}</span>}
              </div>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
