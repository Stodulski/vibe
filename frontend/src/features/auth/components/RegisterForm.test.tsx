import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
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

describe('RegisterForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders step 1 fields (email) and hides later steps', () => {
    renderWithProviders(<RegisterForm />);
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/^nombre$/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/apellido/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^contrase.a$/i)).not.toBeInTheDocument();
  });

  it('renders a "Siguiente" button, not the submit button, on step 1', () => {
    renderWithProviders(<RegisterForm />);
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /crear cuenta/i })).not.toBeInTheDocument();
  });

  it('shows link to login page', () => {
    renderWithProviders(<RegisterForm />);
    expect(screen.getByText(/inici. sesi.n/i)).toBeInTheDocument();
  });
});
