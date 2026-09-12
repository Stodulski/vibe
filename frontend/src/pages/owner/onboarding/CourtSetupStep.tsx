import { PriceConfig, type CreateCourtDto } from '@/features/courts';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtList } from './court-setup-step/CourtList';
import { CourtCreateForm } from './court-setup-step/CourtCreateForm';
import { StepNavigation } from './court-setup-step/StepNavigation';
import { useCourtSetupForm, type MutationLike } from './court-setup-step/useCourtSetupForm';
import type { Court } from '@/shared/types/api.types';

type CourtLike = Pick<Court, 'id' | 'name' | 'sport' | 'court_type'>;

const t = ES_AR;

interface CourtSetupStepProps {
  complexId: string;
  courts: CourtLike[];
  hasCourts: boolean;
  createCourt: MutationLike<{ court: Court }, CreateCourtDto>;
  deleteCourt: MutationLike<unknown, string>;
  onBack: () => void;
  onNext: () => void;
}

export function CourtSetupStep({
  complexId,
  courts,
  hasCourts,
  createCourt,
  deleteCourt,
  onBack,
  onNext,
}: CourtSetupStepProps) {
  const { courtForm, handleCourtSubmit, pricingCourt, setPricingCourt } = useCourtSetupForm(createCourt);

  return (
    <div className="space-y-4">
      {/* Step header card */}
      <div>
        <div className="mb-5 space-y-1.5 sm:mb-6">
          {/* No heading: the step indicator above already names this step,
              and reading it twice is the reader being told what they
              just read. The line that carries information stays. */}
          <p className="text-text-secondary text-sm">{t.complex.onboardingStep2Description}</p>
        </div>

        {hasCourts && (
          <CourtList
            courts={courts}
            onRemove={(courtId) => {
              deleteCourt.mutate(courtId);
            }}
            removePending={deleteCourt.isPending}
          />
        )}

        <CourtCreateForm
          hasCourts={hasCourts}
          onSubmit={handleCourtSubmit}
          register={courtForm.register}
          setValue={courtForm.setValue}
          errors={courtForm.formState.errors}
          isPending={createCourt.isPending}
        />
      </div>

      <StepNavigation onBack={onBack} onNext={onNext} nextDisabled={!hasCourts} />

      {pricingCourt && (
        <PriceConfig
          open={!!pricingCourt}
          onClose={() => {
            setPricingCourt(null);
          }}
          complexId={complexId}
          court={pricingCourt}
        />
      )}
    </div>
  );
}
