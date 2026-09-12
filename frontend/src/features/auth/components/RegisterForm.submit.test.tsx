import { describe, it, expect, vi, beforeEach } from 'vitest';
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

/** Fills step 1 (email) and advances to step 2 (name/last name/phone). */
async function completeStep1(user: ReturnType<typeof userEvent.setup>, email = 'juan@test.com') {
  await user.type(screen.getByLabelText(/email/i), email);
  await user.click(screen.getByRole('button', { name: /siguiente/i }));
  await waitFor(() => {
    expect(screen.getByLabelText(/^nombre$/i)).toBeInTheDocument();
  });
}

/** Fills step 2 (name/last name/phone) and advances to step 3 (password). */
async function completeStep2(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/^nombre$/i), 'Juan');
  await user.type(screen.getByLabelText(/apellido/i), 'Perez');
  await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
  await user.click(screen.getByRole('button', { name: /siguiente/i }));
  await waitFor(() => {
    expect(screen.getByLabelText(/^contrase.a$/i)).toBeInTheDocument();
  });
}

describe('RegisterForm submission', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows validation error for invalid email and does not advance to step 2', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);

    await user.type(screen.getByLabelText(/email/i), 'notanemail');
    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    await waitFor(() => {
      expect(screen.getByText(/email inv.lido/i)).toBeInTheDocument();
    });
    expect(screen.queryByLabelText(/^nombre$/i)).not.toBeInTheDocument();
  });

  it('shows validation error for short password on step 3', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);
    await completeStep1(user);
    await completeStep2(user);

    await user.type(screen.getByLabelText(/^contrase.a$/i), 'short');
    await user.type(screen.getByLabelText(/confirmar contrase.a/i), 'short');
    await user.click(screen.getByRole('button', { name: /crear cuenta/i }));

    await waitFor(() => {
      expect(screen.getByText(/m.nimo 8 caracteres/i)).toBeInTheDocument();
    });
  });

  it('calls mutate with valid registration data after completing all 3 steps', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);
    await completeStep1(user);
    await completeStep2(user);

    await user.type(screen.getByLabelText(/^contrase.a$/i), 'password123');
    await user.type(screen.getByLabelText(/confirmar contrase.a/i), 'password123');
    await user.click(screen.getByRole('button', { name: /crear cuenta/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith({
        first_name: 'Juan',
        last_name: 'Perez',
        email: 'juan@test.com',
        phone: '+541123456789',
        password: 'password123',
      });
    });
  });

  it('lets the user go back to a previous step and keeps its data', async () => {
    const user = userEvent.setup();
    renderWithProviders(<RegisterForm />);
    await completeStep1(user);

    await user.click(screen.getByRole('button', { name: /volver/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/email/i)).toHaveValue('juan@test.com');
    });
  });
});
