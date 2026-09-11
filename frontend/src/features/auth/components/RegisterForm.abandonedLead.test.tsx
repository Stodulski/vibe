import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RegisterForm } from './RegisterForm';
import { renderWithProviders } from '@/test/test-utils';
import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from '../api/leads.api';

vi.mock('../hooks/useRegister', () => ({
  useRegister: () => ({ mutate: vi.fn(), isPending: false, isError: false, error: null }),
}));

vi.mock('../api/leads.api', () => ({
  captureAbandonedRegistrationLead: vi.fn(),
  captureAbandonedRegistrationLeadBeacon: vi.fn(),
}));

describe('RegisterForm abandoned-registration lead capture', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('captures the email once the user advances past step 1', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(captureAbandonedRegistrationLead).toHaveBeenCalledWith('juan@test.com');
    });
    expect(captureAbandonedRegistrationLead).toHaveBeenCalledTimes(1);
  });

  it('does not capture an invalid email', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'notanemail');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(screen.getByText(/email inv.lido/i)).toBeInTheDocument();
    });
    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
  });

  it('does not capture again when the user goes back to step 1 and re-advances', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /volver/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /siguiente/i }));
    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });

    expect(captureAbandonedRegistrationLead).toHaveBeenCalledTimes(1);
  });

  it('captures via sendBeacon on pagehide even while the document is still reported visible (bfcache)', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@test.com');
    expect(document.visibilityState).toBe('visible');
    window.dispatchEvent(new Event('pagehide'));

    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledWith('juan@test.com');
    expect(captureAbandonedRegistrationLeadBeacon).toHaveBeenCalledTimes(1);
    expect(captureAbandonedRegistrationLead).not.toHaveBeenCalled();
  });
});
