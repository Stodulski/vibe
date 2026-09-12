import { Link } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { GoogleSignInSection } from '../GoogleSignInSection';

const t = ES_AR;

interface LoginFormFooterProps {
  isPending: boolean;
  /** `isPending`, plus a Turnstile challenge configured but not yet solved. Defaults to `isPending`. */
  submitDisabled?: boolean;
}

export function LoginFormFooter({ isPending, submitDisabled = isPending }: LoginFormFooterProps) {
  return (
    <>
      <div className="flex justify-end">
        <Link
          to="/forgot-password"
          className="text-primary-400 hover:text-primary-300 text-xs font-medium underline-offset-4 transition-colors hover:underline"
        >
          {t.auth.forgotPasswordLink}
        </Link>
      </div>

      <div className="auth-stagger-3">
        <Button
          type="submit"
          className="h-11 w-full rounded-full font-semibold transition-colors hover:brightness-110"
          disabled={submitDisabled}
        >
          {isPending ? (
            <>
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              <span className="sr-only" aria-live="polite">
                {t.auth.loggingIn}
              </span>
            </>
          ) : (
            t.auth.login
          )}
        </Button>
      </div>

      <GoogleSignInSection />

      <p className="auth-stagger-4 text-text-tertiary !mt-4 text-center text-sm">
        {t.auth.noAccount}{' '}
        <Link
          to="/register"
          className="text-primary-400 hover:text-primary-300 font-medium underline-offset-4 transition-colors hover:underline"
        >
          {t.auth.registerHere}
        </Link>
      </p>
    </>
  );
}
