import { LoadingButton } from '@/shared/components/common/LoadingButton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface GoogleCompleteSubmitProps {
  isPending: boolean;
}

export function GoogleCompleteSubmit({ isPending }: GoogleCompleteSubmitProps) {
  return (
    <LoadingButton
      type="submit"
      className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
      loading={isPending}
      loadingText={t.auth.creatingAccount}
    >
      {t.auth.googleCompleteSubmit}
    </LoadingButton>
  );
}
