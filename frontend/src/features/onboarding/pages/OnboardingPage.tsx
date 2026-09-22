import { Loader2 } from 'lucide-react';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useOnboarding } from './onboarding/useOnboarding';
import { OnboardingStepIndicator } from './onboarding/OnboardingStepIndicator';
import { OnboardingHeaderNav } from './onboarding/OnboardingHeaderNav';
import { OnboardingStepContent } from './onboarding/OnboardingStepContent';
import { MeshBackdrop } from '@/shared/components/layout/MeshBackdrop';

const t = ES_AR;

export default function OnboardingPage() {
  usePageTitle(t.complex.onboardingTitle);

  const state = useOnboarding();
  const { step, animKey, logout } = state;

  // Show a brief loading state while deriving initial step from server data.
  if (step === null) {
    return (
      <div className="bg-bg-base relative flex min-h-[100dvh] items-center justify-center">
        <MeshBackdrop />
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="text-primary-500 size-6 animate-spin" />
          <span className="text-text-tertiary text-sm">{t.common.loading}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-bg-base relative flex min-h-[100dvh] flex-col overflow-x-hidden">
      <MeshBackdrop />
      <OnboardingHeaderNav
        onLogout={() => {
          logout.mutate();
        }}
      />

      <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col px-4 pt-6 pb-8 sm:px-6 sm:pt-10 sm:pb-12">
        {/* Title only. The line under it — "Configurá tu complejo para
            comenzar" — was the kind of sentence that fills a header without
            telling anyone anything they can do, and the step indicator right
            below already says where they are and how far. */}
        <h1 className="font-display text-text-primary mb-6 text-center text-2xl font-bold tracking-tight sm:mb-8 sm:text-3xl">
          {t.complex.onboardingTitle}
        </h1>

        {/* Step indicator */}
        <OnboardingStepIndicator currentStep={step} />

        {/* Step content */}
        <div key={animKey} className="animate-step-in flex-1">
          <OnboardingStepContent state={state} />
        </div>
      </div>
    </div>
  );
}
