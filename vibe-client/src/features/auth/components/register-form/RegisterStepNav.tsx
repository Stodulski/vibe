import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface RegisterStepNavProps {
  onBack?: () => void;
  onNext: () => void;
}

/** Next/Back row for register steps 1-2 (step 3 submits via RegisterFormFooter instead). */
export function RegisterStepNav({ onBack, onNext }: RegisterStepNavProps) {
  return (
    <div className="!mt-4 flex gap-3">
      {onBack && (
        <Button type="button" variant="outline" onClick={onBack} className="h-11 rounded-full font-semibold">
          {t.common.back}
        </Button>
      )}
      <Button
        type="button"
        onClick={onNext}
        className="h-11 flex-1 rounded-full font-semibold transition-colors hover:brightness-110"
      >
        {t.common.next}
      </Button>
    </div>
  );
}
