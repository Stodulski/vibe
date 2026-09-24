import { Button } from '@/shared/components/ui/button';
import { LoadingButton } from '@/shared/components/common/LoadingButton';
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
      <LoadingButton
        type="submit"
        className="h-11 flex-1 rounded-full font-semibold transition-colors hover:brightness-110"
        disabled={submitDisabled}
        loading={isPending}
        loadingText={t.auth.creatingAccount}
      >
        {t.auth.register}
      </LoadingButton>
    </div>
  );
}
