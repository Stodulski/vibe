import { describe, it, expect, vi, beforeEach } from 'vitest';
// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react';
import type { UseFormGetValues } from 'react-hook-form';
import type { RegisterDto } from '../../schemas/auth.schema';

const { mockCaptureBeacon, mockCapture } = vi.hoisted(() => ({
  mockCaptureBeacon: vi.fn(),
  mockCapture: vi.fn(),
}));

vi.mock('../../api/leads.api', () => ({
  captureAbandonedRegistrationLead: mockCapture,
  captureAbandonedRegistrationLeadBeacon: mockCaptureBeacon,
}));

import { useAbandonedRegistrationLead } from './useAbandonedRegistrationLead';

/** A `getValues` stub whose returned fields can change between calls, the way a real form's would as someone types. */
function makeGetValues(initial: Partial<RegisterDto>): {
  getValues: UseFormGetValues<RegisterDto>;
  set: (v: Partial<RegisterDto>) => void;
} {
  let values = initial;
  const getValues = ((field?: string) =>
    field ? (values as Record<string, unknown>)[field] : values) as UseFormGetValues<RegisterDto>;
  return { getValues, set: (v) => (values = { ...values, ...v }) };
}

describe('useAbandonedRegistrationLead', () => {
  beforeEach(() => {
    mockCapture.mockClear();
    mockCaptureBeacon.mockClear();
  });

  it('captures the freshest field values on unmount, not the ones from first render', () => {
    // This is the behavior CI-02 protects: `captureIfPending` is wrapped in
    // `useEffectEvent` instead of being a plain closure the effect silenced
    // exhaustive-deps for, and useEffectEvent's whole point is reading the
    // latest render's values without re-running the effect — so a value
    // typed after mount must still show up in the captured lead.
    const { getValues, set } = makeGetValues({ email: 'first@test.com' });
    const { unmount, rerender } = renderHook(() => useAbandonedRegistrationLead(getValues));

    set({ email: 'juan@test.com', phone: '+5411' });
    rerender();

    unmount();

    expect(mockCapture).toHaveBeenCalledTimes(1);
    expect(mockCapture).toHaveBeenCalledWith({
      email: 'juan@test.com',
      first_name: undefined,
      last_name: undefined,
      phone: '+5411',
    });
  });

  it('does not capture on unmount once markRegistered was called', () => {
    const { getValues } = makeGetValues({ email: 'juan@test.com' });
    const { result, unmount } = renderHook(() => useAbandonedRegistrationLead(getValues));

    result.current.markRegistered();
    unmount();

    expect(mockCapture).not.toHaveBeenCalled();
    expect(mockCaptureBeacon).not.toHaveBeenCalled();
  });

  it('captures via sendBeacon on pagehide, and not again on unmount', () => {
    const { getValues } = makeGetValues({ email: 'juan@test.com' });
    const { unmount } = renderHook(() => useAbandonedRegistrationLead(getValues));

    window.dispatchEvent(new Event('pagehide'));
    unmount();

    expect(mockCaptureBeacon).toHaveBeenCalledTimes(1);
    expect(mockCapture).not.toHaveBeenCalled();
  });

  it('does not capture an invalid email', () => {
    const { getValues } = makeGetValues({ email: 'not-an-email' });
    const { unmount } = renderHook(() => useAbandonedRegistrationLead(getValues));

    unmount();

    expect(mockCapture).not.toHaveBeenCalled();
  });
});
