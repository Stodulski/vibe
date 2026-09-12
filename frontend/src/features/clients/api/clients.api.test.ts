// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { clientsApi } from './clients.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeClient } from '@/test/factories';

async function expectApiResponseError(promise: Promise<unknown>, context: string) {
  try {
    await promise;
    throw new Error('expected promise to reject');
  } catch (err) {
    expect(err).toBeInstanceOf(ApiResponseError);
    expect((err as ApiResponseError).context).toBe(context);
  }
}

describe('clientsApi response validation', () => {
  it('list resolves with a valid response', async () => {
    server.use(
      http.get('*/complexes/:complexId/clients', () =>
        HttpResponse.json({ clients: [makeClient()], metadata: { has_more: false } }),
      ),
    );
    const result = await clientsApi.list('c1');
    expect(result.clients).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when total_bookings is a string', async () => {
    server.use(
      http.get('*/complexes/:complexId/clients', () =>
        HttpResponse.json({
          clients: [{ ...makeClient(), total_bookings: '3' }],
          metadata: { has_more: false },
        }),
      ),
    );
    await expectApiResponseError(clientsApi.list('c1'), 'clientsApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when client is missing', async () => {
    server.use(http.get('*/complexes/:complexId/clients/:clientId', () => HttpResponse.json({ recent_bookings: [] })));
    await expectApiResponseError(clientsApi.getById('c1', 'cl1'), 'clientsApi.getById');
  });
});
