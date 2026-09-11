import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, waitFor } from '@/test/test-utils';
import { MPConnectCard } from './MPConnectCard';

const buildMPAuthUrl = vi.fn((..._args: unknown[]) => 'https://mp.test/auth?code=test');
vi.mock('@/shared/lib/mpAuth', () => ({
  generatePKCE: () => Promise.resolve({ verifier: 'test-verifier', challenge: 'test-challenge' }),
  buildMPAuthUrl: (complexId: string, pkce: unknown, appId?: string) => buildMPAuthUrl(complexId, pkce, appId),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: {
    get: vi.fn(),
    delete: vi.fn(),
  },
}));

let mockMpStatus: { connected: boolean; mp_user_id: string | null; app_id?: string } = {
  connected: false,
  mp_user_id: null,
};
let mockMpStatusIsError = false;
const mockRefetch = vi.fn();

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual('@tanstack/react-query');
  return {
    ...actual,
    useQuery: (opts: { queryKey: string[] }) => {
      if (opts.queryKey.some((k) => typeof k === 'string' && k.includes('mp-status'))) {
        if (mockMpStatusIsError) {
          return { data: undefined, isLoading: false, isError: true, refetch: mockRefetch };
        }
        return { data: mockMpStatus, isLoading: false, isError: false, refetch: mockRefetch };
      }
      return { data: undefined, isLoading: false, isError: false, refetch: mockRefetch };
    },
    useMutation: () => ({
      mutate: vi.fn(),
      isPending: false,
    }),
  };
});

describe('MPConnectCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMpStatus = { connected: false, mp_user_id: null };
    mockMpStatusIsError = false;
  });

  // The app id the API can actually exchange a code with comes from
  // GET .../mp/status, not the client's own build-time env value — a client
  // built against a stale id could send the seller through an authorization
  // the API cannot complete.
  it('builds the auth URL with the app_id from mp/status', async () => {
    mockMpStatus = { connected: false, mp_user_id: null, app_id: 'app-from-server' };
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    await waitFor(() => {
      expect(buildMPAuthUrl).toHaveBeenCalledWith('c1', expect.anything(), 'app-from-server');
    });
  });

  it('renders not connected status when disconnected', async () => {
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    // Narrowed to the status text alone: once the auth URL resolves, the
    // connect button's own label also matches a broader "conectar" pattern,
    // which made this ambiguous once ConnectAction stopped rendering nothing
    // during the brief window before the URL is ready (see ConnectAction.tsx).
    await waitFor(() => {
      expect(screen.getByText(/no conectado/i)).toBeInTheDocument();
    });
  });

  it('renders connect button when not connected', async () => {
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    await waitFor(() => {
      expect(screen.getByRole('link', { name: /conectar/i })).toBeInTheDocument();
    });
  });

  it('renders connected status with user id', async () => {
    mockMpStatus = { connected: true, mp_user_id: 'MP-123456' };
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    await waitFor(() => {
      expect(screen.getByText(/conectado/i)).toBeInTheDocument();
      expect(screen.getByText(/MP-123456/)).toBeInTheDocument();
    });
  });

  it('renders disconnect button when connected', async () => {
    mockMpStatus = { connected: true, mp_user_id: 'MP-123456' };
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /desconectar/i })).toBeInTheDocument();
    });
  });

  it('renders zero cost info badge', async () => {
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);
    await waitFor(() => {
      expect(screen.getByText(/sin cargos adicionales de la plataforma/i)).toBeInTheDocument();
    });
  });

  // Finding M9: a failed `mp/status` used to leave `mpStatus` `undefined`,
  // which reads exactly like "not connected" — the owner saw a disabled
  // "Conectar" button with no indication anything had gone wrong.
  it('renders an error state with retry when the status query fails, not "no conectado"', () => {
    mockMpStatusIsError = true;
    renderWithProviders(<MPConnectCard complexId="c1" province="Buenos Aires" />);

    expect(screen.queryByText(/no conectado/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /conectar/i })).not.toBeInTheDocument();

    const retryButton = screen.getByRole('button', { name: /actualizar/i });
    retryButton.click();
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });
});
