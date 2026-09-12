import { Navigate, useLocation } from 'react-router-dom';
import { GoogleCompleteForm, googleCompleteStateSchema } from '@/features/auth';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export default function GoogleCompletePage() {
  usePageTitle(t.auth.googleCompleteTitle);
  const location = useLocation();

  // `location.state` carries the profile token and preview handed off by
  // `useGoogleSignIn`'s `needs_profile` redirect — never the URL, so it
  // can't be replayed from a bookmark or a shared link. A direct visit or a
  // page reload loses it (browser state doesn't survive a hard reload the
  // way `history.state` normally would across a soft navigation), so it
  // isn't guaranteed to match this shape either — safeParse instead of an
  // `as` cast, same reasoning as `loginRedirectStateSchema` (M9).
  const parsedState = googleCompleteStateSchema.safeParse(location.state);

  if (!parsedState.success) {
    return <Navigate to="/register" replace />;
  }

  const { profile_token, profile } = parsedState.data;

  return (
    <AuthSplitLayout>
      <div className="auth-card w-full">
        <div className="flex flex-col items-center px-0 pt-2 pb-1 text-center md:pt-7">
          <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
            {t.auth.googleCompleteTitle}
          </h1>
          <p className="text-text-tertiary mt-2 text-sm leading-relaxed">{t.auth.googleCompleteHint}</p>
        </div>

        <div className="px-0 pt-5 pb-7">
          <GoogleCompleteForm profileToken={profile_token} profile={profile} />
        </div>
      </div>
    </AuthSplitLayout>
  );
}
