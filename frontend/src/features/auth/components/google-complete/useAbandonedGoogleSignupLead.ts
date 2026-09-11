import { useEffect, useRef } from 'react';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../../api/leads.api';
import { emailField } from '@/shared/lib/validations';

/**
 * Captures the Google sign-up's email as a lead if the person never finishes
 * the second step (the phone number). The address came from Google, already
 * verified, so this is a warmer lead than a typed one; the server files it
 * under its own origin (`source: 'google'`).
 *
 * Three ways out of the page, and what each does:
 * - the tab is hidden or closed (`visibilitychange`/`pagehide`): the capture
 *   goes out via sendBeacon, the only delivery that survives an unload;
 * - an in-app navigation (the "already have an account" link) unmounts the
 *   form while the document lives on: a plain fetch is fine there;
 * - the account was created: the caller says so through `markAccountCreated`
 *   before the success path navigates away, and nothing is captured.
 *
 * Guarded so it fires at most once per mount.
 */
export function useAbandonedGoogleSignupLead(email: string) {
  const capturedRef = useRef(false);
  const accountCreatedRef = useRef(false);

  const captureIfPending = (viaBeacon: boolean) => {
    if (capturedRef.current || accountCreatedRef.current) return;
    if (!emailField.safeParse(email).success) return;
    capturedRef.current = true;
    if (viaBeacon) {
      captureAbandonedRegistrationLeadBeacon(email, 'google');
    } else {
      captureAbandonedRegistrationLead(email, 'google');
    }
  };

  useEffect(() => {
    // A pagehide is a departure whatever the visibility state says (a
    // bfcache unload can fire it with the document still "visible"), so it
    // captures unconditionally; visibilitychange only counts when hidden.
    const handleHidden = () => {
      if (document.visibilityState === 'visible') return;
      captureIfPending(true);
    };
    const handlePageHide = () => {
      captureIfPending(true);
    };
    document.addEventListener('visibilitychange', handleHidden);
    window.addEventListener('pagehide', handlePageHide);
    return () => {
      document.removeEventListener('visibilitychange', handleHidden);
      window.removeEventListener('pagehide', handlePageHide);
      captureIfPending(false);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- captureIfPending reads refs and a prop that never changes for a mounted form
  }, [email]);

  return {
    markAccountCreated: () => {
      accountCreatedRef.current = true;
    },
  };
}
