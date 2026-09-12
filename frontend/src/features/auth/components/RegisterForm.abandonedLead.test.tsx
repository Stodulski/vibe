import { act } from 'react';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RegisterForm } from './RegisterForm';
import { renderWithProviders } from '@/test/test-utils';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../api/leads.api';

const { mockUseRegister } = vi.hoisted(() => ({
  mockUseRegister: vi.fn(),
}));

// The mock records the options the form hands the hook, so a test can play
// the one moment these tests care about: `onRegistered`, which the real hook
// fires the instant the account exists on the server.
vi.mock('../hooks/useRegister', () => ({
  useRegister: (options?: { onRegistered?: () => void }) => {
    mockUseRegister(options);
    return { mutate: vi.fn(), isPending: false, isError: false, error: null };
  },
}));

vi.mock('../api/leads.api', () => ({
  captureAbandonedRegistrationLead: vi.fn(),
  captureAbandonedRegistrationLeadBeacon: vi.fn(),
}));

function registeredCallback(): () => void {
  const options = mockUseRegister.mock.lastCall?.[0] as { onRegistered?: () => void } | undefined;
  if (!options?.onRegistered) throw new Error('RegisterForm did not hand useRegister an onRegistered callback');
  return options.onRegistered;
}

describe('RegisterForm abandoned-registration lead capture', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not capture when the person merely advances past step 1', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });

  it('captures the email when the form unmounts without registering (an in-app navigation)', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    unmount();

    // The untouched fields read back as '' rather than `undefined` now that the
    // form declares `defaultValues` (FORM-09). The request body is unchanged:
    // `leadBody` (leads.api.ts) already drops any field that trims to empty.
    expect(captureAbandonedRegistrationLead).toHaveBeenCalledWith({
      email: 'juan@test.com',
      first_name: '',
      last_name: '',
      phone: '',
    });
    expect(captureAbandonedRegistrationLead).toHaveBeenCalledTimes(1);
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });

  it('captures the name and phone filled in so far when leaving from step 2', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });
    await user.type(screen.getByLabelText(/^nombre$/i), 'Juan');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    unmount();

    expect(captureAbandonedRegistrationLead).toHaveBeenCalledTimes(1);
    const [lead] = vi.mocked(captureAbandonedRegistrationLead).mock.calls[0] ?? [];
    expect(lead).toMatchObject({ email: 'juan@test.com', first_name: 'Juan', phone: '+541123456789' });
    expect(lead?.last_name ?? '').toBe('');
  });

  it('does not capture an invalid email', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'notanemail');
    unmount();

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });
});

describe('RegisterForm abandoned-registration lead capture — unload and success', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('captures via sendBeacon on pagehide from a later step, and only once', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });

    // A bfcache unload can fire pagehide with the document still "visible".
    expect(document.visibilityState).toBe('visible');
    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });

    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledWith(
      expect.objectContaining({ email: 'juan@test.com' }),
    );
    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledTimes(1);

    unmount();
    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledTimes(1);
  });

  it('captures nothing once the account was registered, even when the form then unmounts', async () => {
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    act(() => {
      registeredCallback()();
    });
    act(() => {
      window.dispatchEvent(new Event('pagehide'));
    });
    unmount();

    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
    expect(captureAbandonedRegistrationLeadBeacon).not.toHaveBeenCalled();
  });
});
