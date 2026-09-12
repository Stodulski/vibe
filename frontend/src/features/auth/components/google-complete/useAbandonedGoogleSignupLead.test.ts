import { describe, it, expect, vi, beforeEach } from 'vitest';
// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react';
import type { UseFormGetValues } from 'react-hook-form';
import type { GoogleCompleteDto } from '../../schemas/auth.schema';

const { mockCaptureBeacon, mockCapture } = vi.hoisted(() => ({
  mockCaptureBeacon: vi.fn(),
  mockCapture: vi.fn(),
}));

vi.mock('../../api/leads.api', () => ({
  captureAbandonedRegistrationLead: mockCapture,
  captureAbandonedRegistrationLeadBeacon: mockCaptureBeacon,
}));

import { useAbandonedGoogleSignupLead } from './useAbandonedGoogleSignupLead';

/** A `getValues` stub whose returned fields can change between calls, the way a real form's would as someone types. */
function makeGetValues(initial: Partial<GoogleCompleteDto>): {
  getValues: UseFormGetValues<GoogleCompleteDto>;
  set: (v: Partial<GoogleCompleteDto>) => void;
} {
  let values = initial;
  const getValues = ((field?: string) =>
    field ? (values as Record<string, unknown>)[field] : values) as UseFormGetValues<GoogleCompleteDto>;
  return { getValues, set: (v) => (values = { ...values, ...v }) };
}

describe('useAbandonedGoogleSignupLead', () => {
  beforeEach(() => {
    mockCapture.mockClear();
    mockCaptureBeacon.mockClear();
  });

  it('captures the freshest field values on unmount, not the ones from first render', () => {
    // Same guarantee as the register-form hook: `captureIfPending` is a
    // `useEffectEvent`, so it must see a phone typed after mount, not the
    // value that was current when the effect (deps: [email]) last ran.
    const { getValues, set } = makeGetValues({ first_name: 'Juan' });
    const { rerender, unmount } = renderHook(
      ({ email }: { email: string }) => useAbandonedGoogleSignupLead(email, getValues),
      {
        initialProps: { email: 'juan@test.com' },
      },
    );

    set({ phone: '+5411' });
    rerender({ email: 'juan@test.com' });

    unmount();

    expect(mockCapture).toHaveBeenCalledTimes(1);
    expect(mockCapture).toHaveBeenCalledWith({
      email: 'juan@test.com',
      source: 'google',
      first_name: 'Juan',
      last_name: undefined,
      phone: '+5411',
    });
  });

  it('reads the email current at capture time when the effect re-runs with a new one', () => {
    // `email` is the only render-scoped value the hook closes over, so this
    // is the case that separates the effect event from the closure it
    // replaced: when the effect re-runs for a new email, the cleanup of the
    // previous run captures. A plain closure would capture the previous
    // render's email there; the effect event reads the one current now.
    const { getValues } = makeGetValues({ first_name: 'Juan' });
    const { rerender, unmount } = renderHook(
      ({ email }: { email: string }) => useAbandonedGoogleSignupLead(email, getValues),
      { initialProps: { email: 'juan@test.com' } },
    );

    rerender({ email: 'ana@test.com' });
    unmount();

    expect(mockCapture).toHaveBeenCalledTimes(1);
    expect(mockCapture).toHaveBeenCalledWith(expect.objectContaining({ email: 'ana@test.com' }));
  });

  it('does not capture on unmount once markAccountCreated was called', () => {
    const { getValues } = makeGetValues({});
    const { result, unmount } = renderHook(
      ({ email }: { email: string }) => useAbandonedGoogleSignupLead(email, getValues),
      {
        initialProps: { email: 'juan@test.com' },
      },
    );

    result.current.markAccountCreated();
    unmount();

    expect(mockCapture).not.toHaveBeenCalled();
    expect(mockCaptureBeacon).not.toHaveBeenCalled();
  });

  it('captures via sendBeacon on pagehide, and not again on unmount', () => {
    const { getValues } = makeGetValues({});
    const { unmount } = renderHook(({ email }: { email: string }) => useAbandonedGoogleSignupLead(email, getValues), {
      initialProps: { email: 'juan@test.com' },
    });

    window.dispatchEvent(new Event('pagehide'));
    unmount();

    expect(mockCaptureBeacon).toHaveBeenCalledTimes(1);
    expect(mockCapture).not.toHaveBeenCalled();
  });
});
