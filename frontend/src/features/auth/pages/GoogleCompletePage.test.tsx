import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import GoogleCompletePage from './GoogleCompletePage';

const mockMutate = vi.fn();
vi.mock('@/features/auth/hooks/useGoogleComplete', () => ({
  useGoogleComplete: () => ({ mutate: mockMutate, isPending: false }),
}));

const PROFILE = { email: 'juan@test.com', first_name: 'Juan', last_name: 'Perez' };

function renderPage(state?: unknown) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path: '/register/google', element: <GoogleCompletePage /> },
      { path: '/register', element: <div>Register page</div> },
    ],
    { initialEntries: [{ pathname: '/register/google', state }] },
  );
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe('GoogleCompletePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('redirects to /register when router state is missing (e.g. a page reload)', async () => {
    renderPage(undefined);

    await waitFor(() => {
      expect(screen.getByText('Register page')).toBeInTheDocument();
    });
  });

  it('shows the Google email read-only and prefills the name carried in router state', () => {
    renderPage({ profile_token: 'a-token', profile: PROFILE });

    expect(screen.getByText(ES_AR.auth.googleCompleteTitle)).toBeInTheDocument();
    expect(screen.getByLabelText(/^email$/i)).toHaveValue('juan@test.com');
    expect(screen.getByLabelText(/^nombre$/i)).toHaveValue('Juan');
    expect(screen.getByLabelText(/apellido/i)).toHaveValue('Perez');
  });

  it('submits the profile_token carried in router state along with the entered phone', async () => {
    const user = userEvent.setup();
    renderPage({ profile_token: 'a-token', profile: PROFILE });

    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('button', { name: /completar registro/i }));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith(
        { profile_token: 'a-token', phone: '+541123456789', first_name: 'Juan', last_name: 'Perez' },
        { onError: expect.any(Function) as unknown },
      );
    });
  });
});
