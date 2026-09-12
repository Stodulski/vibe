import { Link } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** "Already have an account? Sign in" link shown below the register form. */
export function RegisterLoginLink() {
  return (
    <p className="text-text-tertiary !mt-3 text-center text-sm">
      {t.auth.hasAccount}{' '}
      <Link
        to="/login"
        className="text-primary-400 hover:text-primary-300 font-medium underline-offset-4 transition-colors hover:underline"
      >
        {t.auth.loginHere}
      </Link>
    </p>
  );
}
