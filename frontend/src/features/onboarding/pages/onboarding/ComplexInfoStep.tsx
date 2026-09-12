import { ComplexForm, IdentityGroup, LocationGroup } from '@/features/complex';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex } from '@/shared/types/api.types';

const t = ES_AR;

interface ComplexInfoStepProps {
  currentComplex: Complex | null;
  onComplexCreated: (created?: Complex) => void;
}

/**
 * The first thing a new owner is asked for: who the club is and where.
 *
 * It renders TWO of the complex form's four groups. It used to render all of
 * them — the very component settings uses — which meant nineteen controls
 * before anyone could create anything: a deposit percentage, a cancellation
 * window and twelve service checkboxes, decided by someone who has not yet
 * added a single court.
 *
 * What is left out is not lost. The deposit and the cancellation window come
 * with defaults the schema already supplies (30% and 24h), the services are
 * optional by definition, and every one of them is a field in Configuración —
 * which the last step of this wizard now names on the way out.
 */
export function ComplexInfoStep({ currentComplex, onComplexCreated }: ComplexInfoStepProps) {
  return (
    <div>
      {/* Steps 2 and 3 opened with a heading and a line saying what they were
          for; step 1 opened with a bare input. It is also the place to say
          what to have at hand before starting, which is the one thing about
          this wizard nobody could know in advance: the club's address. */}
      <div className="mb-5 space-y-1.5 sm:mb-6">
        {/* No heading: the step indicator above already names this step,
            and reading it twice is the reader being told what they
            just read. The line that carries information stays. */}
        <p className="text-text-secondary text-sm">{t.complex.onboardingStep1Description}</p>
      </div>

      <ComplexForm
        complex={currentComplex ?? undefined}
        onSuccess={onComplexCreated}
        fields={(props) => (
          <>
            <IdentityGroup {...props} />
            <LocationGroup form={props.form} />
          </>
        )}
      />
    </div>
  );
}
