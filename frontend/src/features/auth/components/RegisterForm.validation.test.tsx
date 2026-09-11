import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RegisterForm } from './RegisterForm';
import { renderWithProviders } from '@/test/test-utils';

const mockMutate = vi.fn();

vi.mock('../hooks/useRegister', () => ({
  useRegister: () => ({
    mutate: mockMutate,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

vi.mock('../api/leads.api', () => ({
  captureAbandonedRegistrationLead: vi.fn(),
  captureAbandonedRegistrationLeadBeacon: vi.fn(),
}));

describe('RegisterForm validation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('blocks step 1 and shows an error for empty email', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(screen.getByText(/el email es requerido/i)).toBeInTheDocument();
    });
    // Still on step 1 — name field from step 2 never rendered.
    expect(screen.queryByLabelText(/^nombre$/i)).not.toBeInTheDocument();
  });

  it('blocks step 2 and shows an error for empty first name', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'juan@example.com');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(screen.getByText(/el nombre es requerido/i)).toBeInTheDocument();
    });
    expect(screen.queryByLabelText(/^contrase.a$/i)).not.toBeInTheDocument();
  });
});
