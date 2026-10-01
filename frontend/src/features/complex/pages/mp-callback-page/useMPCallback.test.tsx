import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createWrapper } from '@/test/test-utils';
import { consumeMPOAuthSession, saveMPOAuthSession } from '@/shared/lib/mpOAuthSession';
import { useMPCallback } from './useMPCallback';

const navigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => navigate };
});

interface ConnectBody {
  code: string;
  redirect_uri: string;
  code_verifier?: string;
}

// Records every POST to mp/connect and answers with `status`.
function mockConnect(status = 200) {
  const calls: { complexId: string; body: ConnectBody }[] = [];
  server.use(
    http.post('*/complexes/:complexId/mp/connect', async ({ request, params }) => {
      calls.push({ complexId: String(params.complexId), body: (await request.json()) as ConnectBody });
      if (status === 200) return HttpResponse.json({ connected: true, mp_user_id: 'MP-1' });
      return HttpResponse.json({ type: 'about:blank', title: 'nope', status }, { status });
    }),
  );
  return calls;
}

function renderCallback(query: string) {
  return renderHook(() => useMPCallback(), { wrapper: createWrapper([`/settings/mp/callback${query}`]) });
}

beforeEach(() => {
  navigate.mockClear();
  sessionStorage.clear();
});

describe('useMPCallback success path', () => {
  it('posts the verifier saved for the given state, then redirects to the saved return path', async () => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/onboarding' });
    const calls = mockConnect();

    const { result } = renderCallback('?code=abc&state=state-1');

    await waitFor(() => {
      expect(result.current.status).toBe('success');
    });
    expect(calls).toEqual([
      {
        complexId: 'c1',
        body: {
          code: 'abc',
          redirect_uri: `${window.location.origin}/settings/mp/callback`,
          code_verifier: 'verifier-1',
        },
      },
    ]);
    expect(result.current.returnPath).toBe('/onboarding');
    // The success redirect is delayed 1.5s so the confirmation can be read.
    await waitFor(
      () => {
        expect(navigate).toHaveBeenCalledWith('/onboarding', { replace: true });
      },
      { timeout: 3000 },
    );
  });

  it('uses the verifier of its own state when two attempts were prepared (two tabs)', async () => {
    saveMPOAuthSession('state-a', { complexId: 'c1', codeVerifier: 'verifier-a', returnPath: '/settings' });
    saveMPOAuthSession('state-b', { complexId: 'c2', codeVerifier: 'verifier-b', returnPath: '/settings' });
    const calls = mockConnect();

    const first = renderCallback('?code=code-a&state=state-a');
    await waitFor(() => {
      expect(first.result.current.status).toBe('success');
    });
    const second = renderCallback('?code=code-b&state=state-b');
    await waitFor(() => {
      expect(second.result.current.status).toBe('success');
    });

    expect(calls.map((c) => [c.complexId, c.body.code, c.body.code_verifier])).toEqual([
      ['c1', 'code-a', 'verifier-a'],
      ['c2', 'code-b', 'verifier-b'],
    ]);
  });
});

describe('useMPCallback single use', () => {
  it('removes the entry before the exchange runs, so a reload cannot replay it', async () => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/settings' });
    let sawEntryDuringPost = true;
    server.use(
      http.post('*/complexes/:complexId/mp/connect', () => {
        sawEntryDuringPost = sessionStorage.getItem('mp_oauth_state-1') !== null;
        return HttpResponse.json({ connected: true, mp_user_id: 'MP-1' });
      }),
    );

    const { result } = renderCallback('?code=abc&state=state-1');
    // Gone as soon as the hook mounted, before the response arrives.
    expect(sessionStorage.getItem('mp_oauth_state-1')).toBeNull();
    await waitFor(() => {
      expect(result.current.status).toBe('success');
    });
    expect(sawEntryDuringPost).toBe(false);

    // A reload of the same callback URL finds nothing and sends nothing.
    const calls = mockConnect();
    const reloaded = renderCallback('?code=abc&state=state-1');
    expect(reloaded.result.current.status).toBe('error');
    expect(calls).toHaveLength(0);
  });
});

describe('useMPCallback without a usable attempt', () => {
  it('errors with a retry path for an unknown state, without calling the API or redirecting', async () => {
    const calls = mockConnect();

    const { result } = renderCallback('?code=abc&state=unknown');

    expect(result.current.status).toBe('error');
    expect(result.current.errorReason).toBe('expired');
    expect(result.current.returnPath).toBe('/settings');
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(calls).toHaveLength(0);
    expect(navigate).not.toHaveBeenCalled();
  });

  it('errors without calling the API when MercadoPago returns no code, and still clears the entry', () => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/settings' });
    const calls = mockConnect();

    const { result } = renderCallback('?error=access_denied&state=state-1');

    expect(result.current.status).toBe('error');
    expect(result.current.errorReason).toBe('denied');
    expect(calls).toHaveLength(0);
    expect(consumeMPOAuthSession('state-1')).toBeNull();
  });

  it('reports an unknown state without a denial as expired, not denied', () => {
    const { result } = renderCallback('?error=server_error&state=nope');

    expect(result.current.errorReason).toBe('expired');
  });
});

describe('useMPCallback API errors', () => {
  it('reports a conflict for a 409 and stays on the page', async () => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/settings' });
    mockConnect(409);

    const { result } = renderCallback('?code=abc&state=state-1');

    await waitFor(() => {
      expect(result.current.status).toBe('error');
    });
    expect(result.current.errorReason).toBe('conflict');
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(navigate).not.toHaveBeenCalled();
  });

  it.each([400, 404, 422])('reports an expired link for a %i', async (status) => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/settings' });
    mockConnect(status);

    const { result } = renderCallback('?code=abc&state=state-1');

    await waitFor(() => {
      expect(result.current.status).toBe('error');
    });
    expect(result.current.errorReason).toBe('expired');
  });

  it('reports a generic failure for a 5xx', async () => {
    saveMPOAuthSession('state-1', { complexId: 'c1', codeVerifier: 'verifier-1', returnPath: '/settings' });
    mockConnect(502);

    const { result } = renderCallback('?code=abc&state=state-1');

    await waitFor(() => {
      expect(result.current.status).toBe('error');
    });
    expect(result.current.errorReason).toBe('failed');
  });
});
