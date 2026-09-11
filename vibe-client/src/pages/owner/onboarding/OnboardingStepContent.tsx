import { ComplexInfoStep } from './ComplexInfoStep';
import { CourtSetupStepContainer } from './CourtSetupStepContainer';
import { MPConnectStepContainer } from './MPConnectStepContainer';
import type { useOnboarding } from './useOnboarding';

interface OnboardingStepContentProps {
  state: ReturnType<typeof useOnboarding>;
}

export function OnboardingStepContent({ state }: OnboardingStepContentProps) {
  const { step, complexId } = state;

  if (step === 1) {
    return <ComplexInfoStep currentComplex={state.currentComplex} onComplexCreated={state.handleComplexCreated} />;
  }

  if (step === 2 && complexId) {
    return (
      <CourtSetupStepContainer
        complexId={complexId}
        courts={state.courts}
        hasCourts={state.hasCourts}
        onBack={() => {
          state.changeStep(1);
        }}
        onNext={() => {
          state.changeStep(3);
        }}
      />
    );
  }

  if (step === 3 && complexId) {
    return (
      <MPConnectStepContainer
        complexId={complexId}
        completeOnboarding={state.completeOnboarding}
        onBack={() => {
          state.changeStep(2);
        }}
      />
    );
  }

  return null;
}
