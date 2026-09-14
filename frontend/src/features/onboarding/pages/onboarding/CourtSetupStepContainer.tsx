import { useDeleteCourt } from '@/features/courts';
import { CourtSetupStep } from './CourtSetupStep';
import type { Court } from '@/shared/types/api.types';

type CourtLike = Pick<Court, 'id' | 'name' | 'sport' | 'court_type'>;

interface CourtSetupStepContainerProps {
  complexId: string;
  courts: CourtLike[];
  hasCourts: boolean;
  onBack: () => void;
  onNext: () => void;
}

/**
 * Owns the delete mutation, taking a resolved `complexId` — only mounted from
 * `OnboardingStepContent` once one exists, so unlike the previous
 * `useOnboarding()`-level wiring, there's no "no complex yet" state here to
 * fall back for with `?? ''`.
 *
 * Creating is not wired here: `CourtForm` runs its own `useCreateCourt`, the
 * same one the courts page gets, so there is a single create path rather than
 * two that have to stay in step.
 */
export function CourtSetupStepContainer({
  complexId,
  courts,
  hasCourts,
  onBack,
  onNext,
}: CourtSetupStepContainerProps) {
  const deleteCourt = useDeleteCourt(complexId);

  return (
    <CourtSetupStep
      complexId={complexId}
      courts={courts}
      hasCourts={hasCourts}
      deleteCourt={deleteCourt}
      onBack={onBack}
      onNext={onNext}
    />
  );
}
