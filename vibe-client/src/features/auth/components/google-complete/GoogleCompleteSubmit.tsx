import { Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface GoogleCompleteSubmitProps {
  isPending: boolean;
}

export function GoogleCompleteSubmit({ isPending }: GoogleCompleteSubmitProps) {
  return (
    <Button
      type="submit"
      className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
      disabled={isPending}
    >
      {isPending ? (
        <>
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          <span className="sr-only" aria-live="polite">
            {t.auth.creatingAccount}
          </span>
        </>
      ) : (
        t.auth.googleCompleteSubmit
      )}
    </Button>
  );
}
