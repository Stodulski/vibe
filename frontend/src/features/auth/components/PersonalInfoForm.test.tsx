import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { PersonalInfoForm } from './PersonalInfoForm';

// DATA-11: the form reads the session from `useAuth`'s query cache.
vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({
    user: {
      id: 'u1',
      first_name: 'Juan',
      last_name: 'Perez',
      email: 'juan@test.com',
      phone: '1155550000',
    },
    isLoading: false,
    isAuthenticated: true,
  }),
}));

vi.mock('../api/auth.api', () => ({
  authApi: {
    updateMe: vi.fn(),
  },
}));

describe('PersonalInfoForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders first name input with default value', () => {
    renderWithProviders(<PersonalInfoForm />);
    const input = screen.getByLabelText(/nombre/i);
    expect(input).toHaveValue('Juan');
  });

  it('renders last name input with default value', () => {
    renderWithProviders(<PersonalInfoForm />);
    const input = screen.getByLabelText(/apellido/i);
    expect(input).toHaveValue('Perez');
  });

  it('renders email input with default value', () => {
    renderWithProviders(<PersonalInfoForm />);
    const input = screen.getByLabelText(/email/i);
    expect(input).toHaveValue('juan@test.com');
  });

  it('renders phone input with default value', () => {
    renderWithProviders(<PersonalInfoForm />);
    const input = screen.getByLabelText(/tel.fono/i);
    expect(input).toHaveValue('1155550000');
  });

  it('renders save button', () => {
    renderWithProviders(<PersonalInfoForm />);
    expect(screen.getByRole('button', { name: /guardar/i })).toBeInTheDocument();
  });

  it('shows validation error when first name is cleared', async () => {
    const user = userEvent.setup();
    renderWithProviders(<PersonalInfoForm />);

    const firstNameInput = screen.getByLabelText(/nombre/i);
    await user.clear(firstNameInput);
    await user.click(screen.getByRole('button', { name: /guardar/i }));

    expect(await screen.findByText(/el nombre es requerido/i)).toBeInTheDocument();
  });
});
