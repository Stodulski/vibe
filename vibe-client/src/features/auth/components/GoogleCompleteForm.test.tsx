import { act } from 'react';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { GoogleCompleteForm } from './GoogleCompleteForm';
import { renderWithProviders } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';

const mockMutate = vi.fn();
vi.mock('../hooks/useGoogleComplete', () => ({
  useGoogleComplete: () => ({ mutate: mockMutate, isPending: false }),
}));

const PROFILE = { email: 'juan@test.com', first_name: 'Juan', last_name: 'Perez' };

describe('GoogleCompleteForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the Google email read-only and prefills first/last name from the profile', () => {
    renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    const emailInput = screen.getByLabelText(/^email$/i);
    expect(emailInput).toHaveValue('juan@test.com');
    expect(emailInput).toHaveAttribute('readonly');

    expect(screen.getByLabelText(/^nombre$/i)).toHaveValue('Juan');
    expect(screen.getByLabelText(/apellido/i)).toHaveValue('Perez');
  });

  it('requires a phone number before submit', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    await user.click(screen.getByRole('button', { name: /completar registro/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('submits profile_token, phone (with the +54 prefix) and the (possibly edited) names', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    await user.clear(screen.getByLabelText(/apellido/i));
    await user.type(screen.getByLabelText(/apellido/i), 'Gómez');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('button', { name: /completar registro/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        { profile_token: 'a-token', phone: '+541123456789', first_name: 'Juan', last_name: 'Gómez' },
        { onError: expect.any(Function) as unknown },
      );
    });
  });

  it('maps a 422 field error from the server onto the matching field', async () => {
    const user = userEvent.setup();
    renderWithProviders(<GoogleCompleteForm profileToken="a-token" profile={PROFILE} />);

    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('button', { name: /completar registro/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalled();
    });

    const [, options] = mockMutate.mock.calls[0] as [unknown, { onError: (error: unknown) => void }];
    const serverError = await makeConsumedHttpError(422, { error: { phone: 'phone_invalid' } });

    act(() => {
      options.onError(serverError);
    });

    await waitFor(() => {
      expect(screen.getByText('phone_invalid')).toBeInTheDocument();
    });
  });
});
