import { Building2, LayoutGrid, CreditCard, Check } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { OnboardingStep } from './useOnboarding';

const t = ES_AR;

const STEPS = [
  { label: t.complex.onboardingStep1, shortLabel: t.complex.onboardingStepShort1, icon: Building2 },
  {
    label: t.complex.onboardingStep2,
    shortLabel: t.complex.onboardingStepShort2,
    icon: LayoutGrid,
  },
  {
    label: t.complex.onboardingStep3,
    shortLabel: t.complex.onboardingStepShort3,
    icon: CreditCard,
  },
] as const;

interface OnboardingStepIndicatorProps {
  currentStep: OnboardingStep;
}

type StepDef = (typeof STEPS)[number];

function OnboardingStepIndicatorItem({
  step,
  isFirst,
  isActive,
  isCompleted,
}: {
  step: StepDef;
  isFirst: boolean;
  isActive: boolean;
  isCompleted: boolean;
}) {
  const { label, shortLabel, icon: Icon } = step;

  return (
    <div className="flex flex-1 items-center" aria-current={isActive ? 'step' : undefined}>
      {!isFirst && (
        <div className="mx-1.5 h-px flex-1 sm:mx-3">
          <div
            className={cn(
              'h-full rounded-full transition-[background-color] duration-500',
              isCompleted ? 'bg-primary-500' : 'bg-border-subtle',
            )}
          />
        </div>
      )}
      <div className="flex flex-col items-center gap-1.5">
        <div
          className={cn(
            'flex size-9 shrink-0 items-center justify-center rounded-full transition-colors duration-300 sm:size-10',
            isActive
              ? 'bg-primary-500 shadow-brand text-white'
              : isCompleted
                ? 'bg-primary-500/20 text-primary-400'
                : 'bg-bg-elevated text-text-tertiary',
          )}
        >
          {isCompleted ? (
            <Check className="size-4 sm:size-5" strokeWidth={2.5} />
          ) : (
            <Icon className="size-4 sm:size-5" />
          )}
        </div>
        <span
          className={cn(
            'text-xs font-medium transition-colors duration-300',
            isActive ? 'text-text-primary' : isCompleted ? 'text-primary-400' : 'text-text-tertiary',
          )}
        >
          <span className="sm:hidden">{shortLabel}</span>
          <span className="hidden sm:inline">{label}</span>
        </span>
      </div>
    </div>
  );
}

export function OnboardingStepIndicator({ currentStep }: OnboardingStepIndicatorProps) {
  return (
    <nav aria-label={t.complex.onboardingStepProgress} className="mb-8 sm:mb-10">
      <div className="flex items-center justify-between">
        {STEPS.map((step, i) => {
          const stepNum = (i + 1) as OnboardingStep;
          return (
            <OnboardingStepIndicatorItem
              key={step.label}
              step={step}
              isFirst={i === 0}
              isActive={stepNum === currentStep}
              isCompleted={stepNum < currentStep}
            />
          );
        })}
      </div>
    </nav>
  );
}
