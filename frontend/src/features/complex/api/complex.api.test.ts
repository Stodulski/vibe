import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { complexApi } from './complex.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeComplex } from '@/test/factories';

async function expectApiResponseError(promise: Promise<unknown>, context: string) {
  try {
    await promise;
    throw new Error('expected promise to reject');
  } catch (err) {
    expect(err).toBeInstanceOf(ApiResponseError);
    expect((err as ApiResponseError).context).toBe(context);
  }
}

describe('complexApi response validation', () => {
  it('list resolves with a valid response', async () => {
    server.use(http.get('*/complexes', () => HttpResponse.json({ complexes: [makeComplex()] })));
    const result = await complexApi.list();
    expect(result.complexes).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when is_active is a string', async () => {
    server.use(
      http.get('*/complexes', () => HttpResponse.json({ complexes: [{ ...makeComplex(), is_active: 'yes' }] })),
    );
    await expectApiResponseError(complexApi.list(), 'complexApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when complex is missing', async () => {
    server.use(http.get('*/complexes/:complexId', () => HttpResponse.json({})));
    await expectApiResponseError(complexApi.getById('c1'), 'complexApi.getById');
  });

  it('delete rejects with ApiResponseError carrying its context when courts_deactivated is missing', async () => {
    server.use(http.delete('*/complexes/:complexId', () => HttpResponse.json({ message: 'ok' })));
    await expectApiResponseError(complexApi.delete('c1'), 'complexApi.delete');
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('complexApi.connectMP', () => {
  it('posts the OAuth exchange to the complex MP endpoint and resolves with a valid response', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/complexes/:complexId/mp/connect', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ connected: true, mp_user_id: 'mp1' });
      }),
    );

    const result = await complexApi.connectMP('c1', {
      code: 'oauth-code',
      redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
      code_verifier: 'pkce-verifier',
    });

    expect(receivedBody).toEqual({
      code: 'oauth-code',
      redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
      code_verifier: 'pkce-verifier',
    });
    expect(result.connected).toBe(true);
  });

  it('omits code_verifier when the authorize step ran without PKCE', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/complexes/:complexId/mp/connect', async ({ request }) => {
        receivedBody = await request.json();
        // A successful connect always names the account it linked, so the
        // response carries `mp_user_id` even though this test is about the
        // request body.
        return HttpResponse.json({ connected: true, mp_user_id: 'mp1' });
      }),
    );

    await complexApi.connectMP('c1', {
      code: 'oauth-code',
      redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
    });

    expect(receivedBody).toEqual({ code: 'oauth-code', redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback' });
  });

  it('rejects with ApiResponseError carrying its context when connected is not a boolean', async () => {
    server.use(http.post('*/complexes/:complexId/mp/connect', () => HttpResponse.json({ connected: 'yes' })));
    await expectApiResponseError(
      complexApi.connectMP('c1', { code: 'oauth-code', redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback' }),
      'complexApi.connectMP',
    );
  });
});
