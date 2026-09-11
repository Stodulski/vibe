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
  const { step, animKey, isNewComplex, logout, navigate } = state;

  // Show a brief loading state while deriving initial step from server data.
  if (step === null) {
    return (
      <div className="relative flex min-h-[100dvh] items-center justify-center bg-bg-base">
        <MeshBackdrop />
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="size-6 animate-spin text-primary-500" />
          <span className="text-sm text-text-tertiary">{t.common.loading}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="relative flex min-h-[100dvh] flex-col overflow-x-hidden bg-bg-base">
      <MeshBackdrop />
      <OnboardingHeaderNav
        step={step}
        onBack={() => {
          void navigate('/complexes', { state: { from: '/onboarding' } });
        }}
        onLogout={() => {
          logout.mutate();
        }}
      />

      <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col px-4 pb-8 pt-6 sm:px-6 sm:pb-12 sm:pt-10">
        {/* Title only. The line under it — "Configurá tu complejo para
            comenzar" — was the kind of sentence that fills a header without
            telling anyone anything they can do, and the step indicator right
            below already says where they are and how far. */}
        <h1 className="mb-6 text-center font-display text-2xl font-bold tracking-tight text-text-primary sm:mb-8 sm:text-3xl">
          {isNewComplex ? t.complex.onboardingTitleNew : t.complex.onboardingTitle}
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
