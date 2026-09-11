import { Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface RegisterFormFooterProps {
  isPending: boolean;
  /** `isPending`, plus a Turnstile challenge configured but not yet solved. Defaults to `isPending`. */
  submitDisabled?: boolean;
  onBack: () => void;
}

export function RegisterFormFooter({ isPending, submitDisabled = isPending, onBack }: RegisterFormFooterProps) {
  return (
    <div className="auth-stagger-4 !mt-4 flex gap-3">
      <Button
        type="button"
        variant="outline"
        onClick={onBack}
        disabled={isPending}
        className="h-11 rounded-full font-semibold"
      >
        {t.common.back}
      </Button>
      <Button
        type="submit"
        className="h-11 flex-1 rounded-full font-semibold transition-colors hover:brightness-110"
        disabled={submitDisabled}
      >
        {isPending ? (
          <>
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            <span className="sr-only" aria-live="polite">
              {t.auth.creatingAccount}
            </span>
          </>
        ) : (
          t.auth.register
        )}
      </Button>
    </div>
  );
}
