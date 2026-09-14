import { useState } from 'react';
import { Plus } from 'lucide-react';
import { CourtForm, PriceConfig } from '@/features/courts';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtList } from './court-setup-step/CourtList';
import { StepNavigation } from './court-setup-step/StepNavigation';
import type { Court, CourtWithPrices } from '@/shared/types/api.types';

type CourtLike = Pick<Court, 'id' | 'name' | 'sport' | 'court_type'>;

const t = ES_AR;

/** Minimal mutation shape -- avoids coupling to the concrete UseMutationResult generic params. */
interface MutationLike<TVariables> {
  mutate: (variables: TVariables) => void;
  isPending: boolean;
}

interface CourtSetupStepProps {
  complexId: string;
  courts: CourtLike[];
  hasCourts: boolean;
  deleteCourt: MutationLike<string>;
  onBack: () => void;
  onNext: () => void;
}

/**
 * The first court, created through the same dialog the courts page uses.
 *
 * This step used to carry its own inline form -- a second name input, a second
 * pair of sport/type selects and a second submit, none of which the courts page
 * form did differently. Two forms for one court is two places to change a field
 * and one of them to forget: the inline copy had no description input at all,
 * so a court created during onboarding could not be described until the owner
 * went looking for the edit dialog. Reusing `CourtForm` makes the field list
 * the same everywhere by construction, and it already chains into `PriceConfig`
 * through `onCreated`, which is the order the owner needs anyway: a court is
 * not bookable until it has a price.
 */
export function CourtSetupStep({ complexId, courts, hasCourts, deleteCourt, onBack, onNext }: CourtSetupStepProps) {
  const [formOpen, setFormOpen] = useState(false);
  const [pricingCourt, setPricingCourt] = useState<CourtWithPrices | null>(null);

  return (
    <div className="space-y-4">
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

        {/* Full width because it is the only thing to do on this screen until a
            court exists, and a small right-aligned button on an otherwise empty
            step reads as secondary to the navigation below it. */}
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={() => {
            setFormOpen(true);
          }}
        >
          <Plus className="size-4" />
          {hasCourts ? t.complex.addAnotherCourt : t.complex.addFirstCourt}
        </Button>
      </div>

      <StepNavigation onBack={onBack} onNext={onNext} nextDisabled={!hasCourts} />

      <CourtForm
        open={formOpen}
        onClose={() => {
          setFormOpen(false);
        }}
        complexId={complexId}
        onCreated={(court) => {
          setPricingCourt(court);
        }}
      />

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
