// @vitest-environment node
const { mockGet, mockPatch } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPatch: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, patch: mockPatch },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { adminApi } from './admin.api';
import { ApiResponseError } from '@/shared/lib/apiParse';

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

const validStats = {
  total_users: 10,
  active_users: 8,
  new_users_month: 2,
  total_complexes: 5,
  new_complexes_month: 1,
  total_courts: 12,
  total_bookings: 100,
  total_revenue: 500000,
};

describe('adminApi response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('getStats resolves with a valid response', async () => {
    mockGet.mockReturnValue(jsonOf({ stats: validStats }));
    const result = await adminApi.getStats();
    expect(result.stats.total_users).toBe(10);
  });

  it('getStats rejects with ApiResponseError carrying its context when total_revenue is a string', async () => {
    mockGet.mockReturnValue(jsonOf({ stats: { ...validStats, total_revenue: '500000' } }));
    await expectApiResponseError(adminApi.getStats(), 'adminApi.getStats');
  });

  it('toggleUserActive rejects with ApiResponseError carrying its context when message is missing', async () => {
    mockPatch.mockReturnValue(jsonOf({}));
    await expectApiResponseError(adminApi.toggleUserActive('u1', true), 'adminApi.toggleUserActive');
  });
});
