import { useEffect, useRef } from 'react';
import type { UseFormGetValues } from 'react-hook-form';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../../api/leads.api';
import { emailField } from '@/shared/lib/validations';
import type { RegisterDto } from '../../schemas/auth.schemas';

/**
 * Captures the register form's email as a lead if the person never finishes
 * — either they move past step 1 (call `captureOnAdvance` right there) or
 * they abandon the page while still on it (a visibilitychange/pagehide
 * listener, fired via sendBeacon — the only delivery mechanism reliable
 * during an unload). Guarded so it only ever fires once per visit.
 */
export function useAbandonedRegistrationLead(step: number, getValues: UseFormGetValues<RegisterDto>) {
  const capturedRef = useRef(false);

  const captureIfValid = (viaBeacon: boolean) => {
    if (capturedRef.current) return;
    const email = getValues('email');
    if (!emailField.safeParse(email).success) return;
    capturedRef.current = true;
    if (viaBeacon) {
      captureAbandonedRegistrationLeadBeacon(email);
    } else {
      captureAbandonedRegistrationLead(email);
    }
  };

  useEffect(() => {
    if (step !== 1) return;
    const handleAbandon = () => {
      if (document.visibilityState === 'visible') return;
      captureIfValid(true);
    };
    document.addEventListener('visibilitychange', handleAbandon);
    window.addEventListener('pagehide', handleAbandon);
    return () => {
      document.removeEventListener('visibilitychange', handleAbandon);
      window.removeEventListener('pagehide', handleAbandon);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- captureIfValid reads refs/getValues, not reactive state
  }, [step]);

  return {
    captureOnAdvance: () => {
      captureIfValid(false);
    },
  };
}
