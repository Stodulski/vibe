import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import { renderWithProviders, screen, waitFor, within } from '@/test/test-utils';
import { server } from '@/test/msw/server';
import { ES_AR } from '@/shared/i18n/es_AR';
import { InstagramConnectCard } from './InstagramConnectCard';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.instagram;

function status(overrides: Partial<{ connected: boolean; needs_reconnect: boolean; username: string | null }>) {
  server.use(
    http.get('*/complexes/:complexId/instagram', () =>
      HttpResponse.json({
        enabled: true,
        connected: false,
        needs_reconnect: false,
        username: null,
        account_type: null,
        expires_at: null,
        ...overrides,
      }),
    ),
  );
}

let assign: ReturnType<typeof vi.fn<(url: string | URL) => void>>;

function stubNavigation() {
  beforeEach(() => {
    vi.clearAllMocks();
    assign = vi.fn<(url: string | URL) => void>();
    vi.spyOn(window.location, 'assign').mockImplementation(assign);
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });
}

describe('InstagramConnectCard connect', () => {
  stubNavigation();

  it('sends the browser to the authorize_url the server returns', async () => {
    status({});
    const connect = vi.fn();
    server.use(
      http.post('*/complexes/:complexId/instagram/connect', ({ params }) => {
        connect(params.complexId);
        return HttpResponse.json({ authorize_url: 'https://www.instagram.com/oauth/authorize?state=s1' });
      }),
    );
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    await userEvent.click(await screen.findByRole('button', { name: t.connect }));

    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith('https://www.instagram.com/oauth/authorize?state=s1');
    });
    expect(connect).toHaveBeenCalledWith('c1');
  });

  it('says the feature is unavailable on 501 and stays on the page', async () => {
    status({});
    server.use(
      http.post('*/complexes/:complexId/instagram/connect', () =>
        HttpResponse.json({ title: 'Not Implemented' }, { status: 501 }),
      ),
    );
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    await userEvent.click(await screen.findByRole('button', { name: t.connect }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.connectUnavailable);
    });
    expect(assign).not.toHaveBeenCalled();
  });

  it('offers Reconnect (and Disconnect) when the token needs reconnecting, and reconnecting starts a new attempt', async () => {
    status({ needs_reconnect: true, username: 'club.padel' });
    const connect = vi.fn();
    server.use(
      http.post('*/complexes/:complexId/instagram/connect', () => {
        connect();
        return HttpResponse.json({ authorize_url: 'https://www.instagram.com/oauth/authorize?state=s2' });
      }),
    );
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    const reconnect = await screen.findByRole('button', { name: t.reconnect });
    expect(screen.getByRole('button', { name: t.disconnect })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.connect })).not.toBeInTheDocument();

    await userEvent.click(reconnect);
    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith('https://www.instagram.com/oauth/authorize?state=s2');
    });
    expect(connect).toHaveBeenCalledTimes(1);
  });
});

describe('InstagramConnectCard refusals and disconnect', () => {
  stubNavigation();

  it('does not navigate to a non-https authorize_url', async () => {
    status({});
    server.use(
      http.post('*/complexes/:complexId/instagram/connect', () =>
        HttpResponse.json({ authorize_url: 'javascript:alert(1)' }),
      ),
    );
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    await userEvent.click(await screen.findByRole('button', { name: t.connect }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.connectError);
    });
    expect(assign).not.toHaveBeenCalled();
  });

  it('asks for confirmation before disconnecting, and only then calls DELETE', async () => {
    status({ connected: true, username: 'club.padel' });
    const del = vi.fn();
    server.use(
      http.delete('*/complexes/:complexId/instagram', ({ params }) => {
        del(params.complexId);
        return HttpResponse.json({ connected: false });
      }),
    );
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    expect(await screen.findByText('@club.padel')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.connect })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: t.disconnect }));
    const dialog = await screen.findByRole('alertdialog');
    expect(del).not.toHaveBeenCalled();
    const confirm = within(dialog).getByRole('button', { name: /confirmar/i });
    await userEvent.click(confirm);

    await waitFor(() => {
      expect(del).toHaveBeenCalledWith('c1');
    });
    expect(toast.success).toHaveBeenCalledWith(t.disconnectSuccess);
  });

  it('shows a retry instead of "not connected" when the status read fails', async () => {
    server.use(http.get('*/complexes/:complexId/instagram', () => HttpResponse.json({}, { status: 500 })));
    renderWithProviders(<InstagramConnectCard complexId="c1" />);

    expect(await screen.findByRole('button', { name: ES_AR.common.refresh })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.connect })).not.toBeInTheDocument();
  });
});
