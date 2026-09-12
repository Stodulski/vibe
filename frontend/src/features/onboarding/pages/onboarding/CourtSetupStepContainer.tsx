import { useCreateCourt, useDeleteCourt } from '@/features/courts';
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
 * Owns the court mutations, taking a resolved `complexId` — only mounted
 * from `OnboardingStepContent` once one exists, so unlike the previous
 * `useOnboarding()`-level wiring, there's no "no complex yet" state here to
 * fall back for with `?? ''`.
 */
export function CourtSetupStepContainer({
  complexId,
  courts,
  hasCourts,
  onBack,
  onNext,
}: CourtSetupStepContainerProps) {
  const createCourt = useCreateCourt(complexId);
  const deleteCourt = useDeleteCourt(complexId);

  return (
    <CourtSetupStep
      complexId={complexId}
      courts={courts}
      hasCourts={hasCourts}
      createCourt={createCourt}
      deleteCourt={deleteCourt}
      onBack={onBack}
      onNext={onNext}
    />
  );
}
