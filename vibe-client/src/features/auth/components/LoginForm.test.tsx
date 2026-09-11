import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LoginForm } from './LoginForm';
import { renderWithProviders } from '@/test/test-utils';

const mockMutate = vi.fn();

vi.mock('../hooks/useLogin', () => ({
  useLogin: () => ({
    mutate: mockMutate,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

// Two tests are missing on purpose. One asserted the submit button renders,
// which every validation test below already proves by clicking it; the other
// asserted the register link's wording, which has no branch behind it and can
// only fail when someone rewords the copy. What is left renders the form and
// then makes it do something.
describe('LoginForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders email and password inputs', () => {
    renderWithProviders(<LoginForm />);
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/contrase.a/i, { selector: 'input' })).toBeInTheDocument();
  });

  it('shows validation error for empty email on submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.click(screen.getByRole('button', { name: /iniciar sesi.n/i }));

    await waitFor(() => {
      expect(screen.getByText(/el email es requerido/i)).toBeInTheDocument();
    });
  });

  it('shows validation error for empty password on submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.click(screen.getByRole('button', { name: /iniciar sesi.n/i }));

    await waitFor(() => {
      expect(screen.getByText(/campo requerido/i)).toBeInTheDocument();
    });
  });

  // Login must not reveal the password format policy (min length, etc.) —
  // that's for register/reset. A short password should reach the backend
  // and come back as the same generic invalid-credentials error as any
  // other wrong password.
  it('does not enforce a minimum password length on submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/contrase.a/i, { selector: 'input' }), 'x');
    await user.click(screen.getByRole('button', { name: /iniciar sesi.n/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith({
        email: 'test@example.com',
        password: 'x',
      });
    });
  });

  it('calls mutate with valid data', async () => {
    const user = userEvent.setup();
    renderWithProviders(<LoginForm />);

    await user.type(screen.getByLabelText(/email/i), 'test@example.com');
    await user.type(screen.getByLabelText(/contrase.a/i, { selector: 'input' }), 'password123');
    await user.click(screen.getByRole('button', { name: /iniciar sesi.n/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith({
        email: 'test@example.com',
        password: 'password123',
      });
    });
  });
});
