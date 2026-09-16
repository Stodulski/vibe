import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';

const mockConfirmEmailChange = vi.fn<(token: string) => Promise<{ message: string }>>();
const mockLogout = vi.fn();

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: {
    confirmEmailChange: (token: string) => mockConfirmEmailChange(token),
  },
}));

// Selector-aware, like the real zustand store: every call site reads one
// atomic slice now (STORE-02), not the whole state.
vi.mock('@/shared/stores', () => ({
  useStore: (selector: (s: { logout: typeof mockLogout }) => unknown) => selector({ logout: mockLogout }),
}));

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ className }: { className?: string }) => (
    <header data-testid="app-header" className={className}>
      Header
    </header>
  ),
}));

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
}));

async function makeHttpError(status: number, data: unknown): Promise<HTTPError> {
  const bodyText = JSON.stringify(data);
  const response = new Response(bodyText, { status });
  await response.text();
  const request = new Request('https://example.com/test');
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = data;
  return error;
}

function createTestQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

async function renderPage(entries: string[], client: QueryClient = createTestQueryClient()) {
  const ConfirmEmailChangePage = (await import('./ConfirmEmailChangePage')).default;
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={entries}>
        <ConfirmEmailChangePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ConfirmEmailChangePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the confirm prompt with a token, and sends no request until it is clicked', async () => {
    await renderPage(['/confirm-email-change?token=test-token']);
    expect(screen.getByRole('button', { name: 'Confirmar cambio de email' })).toBeInTheDocument();
    expect(mockConfirmEmailChange).not.toHaveBeenCalled();
  });

  it('shows the expired/invalid message when there is no token, and never offers a button', async () => {
    await renderPage(['/confirm-email-change']);
    expect(screen.getByText(/El enlace de confirmación es inválido, expiró o ya fue utilizado/)).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(mockConfirmEmailChange).not.toHaveBeenCalled();
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body.
describe('ConfirmEmailChangePage, success and the single-fire guard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('clicking confirm sends exactly one request, then shows success, logs out and clears the query cache', async () => {
    mockConfirmEmailChange.mockResolvedValue({ message: 'email address updated' });
    const client = createTestQueryClient();
    const clearSpy = vi.spyOn(client, 'clear');
    const user = userEvent.setup();

    await renderPage(['/confirm-email-change?token=valid-token'], client);
    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));

    await waitFor(() => {
      expect(screen.getByText('Tu email fue actualizado')).toBeInTheDocument();
    });
    expect(mockConfirmEmailChange).toHaveBeenCalledTimes(1);
    expect(mockConfirmEmailChange).toHaveBeenCalledWith('valid-token');

    await waitFor(() => {
      expect(mockLogout).toHaveBeenCalledTimes(1);
      expect(clearSpy).toHaveBeenCalled();
    });
  });

  it('two fast clicks send exactly one request (the single-fire guard)', async () => {
    let resolveConfirm!: (value: { message: string }) => void;
    const pending = new Promise<{ message: string }>((resolve) => {
      resolveConfirm = resolve;
    });
    mockConfirmEmailChange.mockReturnValue(pending);
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=double-click-token']);

    const button = screen.getByRole('button', { name: 'Confirmar cambio de email' });
    await user.click(button);
    await user.click(button);

    expect(mockConfirmEmailChange).toHaveBeenCalledTimes(1);
    resolveConfirm({ message: 'ok' });
  });
});

// A separate describe for the same reason as above: this file's per-function
// line budget is counted per describe callback, not per file.
describe('ConfirmEmailChangePage, terminal error mapping (400, 422)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('maps a 400 to the expired/invalid-link message with no retry option', async () => {
    mockConfirmEmailChange.mockRejectedValue(await makeHttpError(400, { title: 'Bad Request' }));
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=bad-token']);

    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));
    await waitFor(() => {
      expect(screen.getByText(/El enlace de confirmación es inválido, expiró o ya fue utilizado/)).toBeInTheDocument();
    });
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('maps a 422 to the "already in use" message with no retry option', async () => {
    mockConfirmEmailChange.mockRejectedValue(await makeHttpError(422, { title: 'Unprocessable Entity' }));
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=taken-token']);

    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));
    await waitFor(() => {
      expect(screen.getByText(/Esa dirección de email ya está en uso por otra cuenta/)).toBeInTheDocument();
    });
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});

describe('ConfirmEmailChangePage, retryable error mapping (429, 5xx, network)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('maps a 429 to the rate-limited message and keeps a retry button', async () => {
    mockConfirmEmailChange.mockRejectedValue(await makeHttpError(429, { title: 'Too Many Requests' }));
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=rate-limited-token']);

    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));
    await waitFor(() => {
      expect(screen.getByText(/Demasiados intentos/)).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();

    mockConfirmEmailChange.mockResolvedValue({ message: 'ok' });
    await user.click(screen.getByRole('button', { name: 'Reintentar' }));
    await waitFor(() => {
      expect(screen.getByText('Tu email fue actualizado')).toBeInTheDocument();
    });
    expect(mockConfirmEmailChange).toHaveBeenCalledTimes(2);
  });

  it('maps a 500 to a generic temporary error, not "expired", and keeps a retry button', async () => {
    mockConfirmEmailChange.mockRejectedValue(await makeHttpError(500, { title: 'Internal Server Error' }));
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=server-error-token']);

    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));
    await waitFor(() => {
      expect(screen.getByText('No pudimos confirmar el cambio en este momento. Probá de nuevo.')).toBeInTheDocument();
    });
    expect(screen.queryByText(/inválido, expiró o ya fue utilizado/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
  });

  it('maps a network failure to the same generic temporary error, with a retry button', async () => {
    mockConfirmEmailChange.mockRejectedValue(new TypeError('Failed to fetch'));
    const user = userEvent.setup();
    await renderPage(['/confirm-email-change?token=network-error-token']);

    await user.click(screen.getByRole('button', { name: 'Confirmar cambio de email' }));
    await waitFor(() => {
      expect(screen.getByText('No pudimos confirmar el cambio en este momento. Probá de nuevo.')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
  });
});
