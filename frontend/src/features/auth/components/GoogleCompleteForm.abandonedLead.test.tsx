import { act } from 'react';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { GoogleCompleteForm } from './GoogleCompleteForm';
import { renderWithProviders } from '@/test/test-utils';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../api/leads.api';

const { mockMutate, mockUseGoogleComplete } = vi.hoisted(() => ({
  mockMutate: vi.fn(),
  mockUseGoogleComplete: vi.fn(),
}));

// The mock behaves like the real hook on the one point these tests care
// about: `mutate` resolves the account creation, so `onAccountCreated` runs.
vi.mock('../hooks/useGoogleComplete', () => ({
  useGoogleComplete: (options?: { onAccountCreated?: () => void }) => {
    mockUseGoogleComplete(options);
    return {
      mutate: (...args: unknown[]) => {
        mockMutate(...args);
        options?.onAccountCreated?.();
      },
      isPending: false,
    };
  },
}));

vi.mock('../api/leads.api', () => ({
  captureAbandonedRegistrationLead: vi.fn(),
  captureAbandonedRegistrationLeadBeacon: vi.fn(),
}));

const PROFILE = { email: 'juan@test.com', first_name: 'Juan', last_name: 'Perez' };

describe('GoogleCompleteForm abandoned-signup lead capture', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('captures the Google email when the form unmounts without an account being created', () => {
    const { unmount } = renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    unmount();

    expect(captureAbandonedRegistrationLead).toHaveBeenCalledWith({
      email: 'juan@test.com',
      source: 'google',
      first_name: 'Juan',
      last_name: 'Perez',
      phone: '',
    });
    expect(captureAbandonedRegistrationLead).toHaveBeenCalledTimes(1);
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });

  it('captures the profile names and the phone typed so far', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    unmount();

    expect(captureAbandonedRegistrationLead).toHaveBeenCalledWith({
      email: 'juan@test.com',
      source: 'google',
      first_name: 'Juan',
      last_name: 'Perez',
      phone: '+541123456789',
    });
  });

  it('captures via sendBeacon on pagehide, and does not capture again on the later unmount', () => {
    const { unmount } = renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });

    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledWith(
      expect.objectContaining({ email: 'juan@test.com', source: 'google' }),
    );
    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledTimes(1);

    unmount();

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledTimes(1);
  });

  it('captures nothing once the account was created, even when the form then unmounts', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('button', { name: /completar registro/i }));

    expect(mockMutate).toHaveBeenCalledTimes(1);

    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    unmount();

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });
});
