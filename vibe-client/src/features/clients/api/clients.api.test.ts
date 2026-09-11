// @vitest-environment node
const { mockGet, mockPut } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPut: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, put: mockPut },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { clientsApi } from './clients.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeClient } from '@/test/factories';

function jsonOf(body: unknown) {
  return { json: vi.fn().mockResolvedValue(body) };
}

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
  beforeEach(() => vi.clearAllMocks());

  it('list resolves with a valid response', async () => {
    mockGet.mockReturnValue(
      jsonOf({
        clients: [makeClient()],
        metadata: { has_more: false },
      }),
    );
    const result = await clientsApi.list('c1');
    expect(result.clients).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when total_bookings is a string', async () => {
    mockGet.mockReturnValue(
      jsonOf({
        clients: [{ ...makeClient(), total_bookings: '3' }],
        metadata: { has_more: false },
      }),
    );
    await expectApiResponseError(clientsApi.list('c1'), 'clientsApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when client is missing', async () => {
    mockGet.mockReturnValue(jsonOf({ recent_bookings: [] }));
    await expectApiResponseError(clientsApi.getById('c1', 'cl1'), 'clientsApi.getById');
  });
});
