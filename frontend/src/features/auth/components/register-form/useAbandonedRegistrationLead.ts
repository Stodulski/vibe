import { useEffect, useRef } from 'react';
import type { UseFormGetValues } from 'react-hook-form';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../../api/leads.api';
import { emailField } from '@/shared/lib/validations';
import type { RegisterDto } from '../../schemas/auth.schemas';

/**
 * Captures the register form's email as a lead if the person leaves without
 * registering, whichever step they are on. The fields are read at capture
 * time, so a valid address typed on step 1 is captured even from step 3, and
 * whatever name and phone were filled in by then travel with it.
 *
 * Three ways out of the form, and what each does:
 * - the tab is hidden or closed (`visibilitychange`/`pagehide`): the capture
 *   goes out via sendBeacon, the only delivery that survives an unload;
 * - an in-app navigation (the "already have an account" link) unmounts the
 *   form while the document lives on: a plain fetch is fine there;
 * - the account was registered: the caller says so through `markRegistered`
 *   before the success path navigates away, and nothing is captured.
 *
 * It used to capture the moment the person advanced past step 1, which put
 * everyone who reached step 2 in the sheet, those who finished included.
 *
 * Guarded so it fires at most once per mount. Same shape as
 * `useAbandonedGoogleSignupLead`.
 */
export function useAbandonedRegistrationLead(getValues: UseFormGetValues<RegisterDto>) {
  const capturedRef = useRef(false);
  const registeredRef = useRef(false);

  const captureIfPending = (viaBeacon: boolean) => {
    if (capturedRef.current || registeredRef.current) return;
    const email = getValues('email');
    if (!emailField.safeParse(email).success) return;
    capturedRef.current = true;
    // Partial by design: the person may have left on any step. The API
    // helper drops the empty ones.
    const lead = {
      email,
      first_name: getValues('first_name'),
      last_name: getValues('last_name'),
      phone: getValues('phone'),
    };
    if (viaBeacon) {
      captureAbandonedRegistrationLeadBeacon(lead);
    } else {
      captureAbandonedRegistrationLead(lead);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- captureIfPending reads refs and getValues, not reactive state
  }, []);

  return {
    markRegistered: () => {
      registeredRef.current = true;
    },
  };
}
