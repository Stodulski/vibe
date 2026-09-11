import { Link } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** "Already have an account? Sign in" link shown below the register form. */
export function RegisterLoginLink() {
  return (
    <p className="!mt-3 text-center text-sm text-text-tertiary">
      {t.auth.hasAccount}{' '}
      <Link
        to="/login"
        className="font-medium text-primary-400 underline-offset-4 transition-colors hover:text-primary-300 hover:underline"
      >
        {t.auth.loginHere}
      </Link>
    </p>
  );
}
