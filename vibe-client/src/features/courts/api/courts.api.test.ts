// @vitest-environment node
const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, post: mockPost, put: mockPut, delete: mockDelete },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { courtsApi } from './courts.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeCourt, makePrice } from '@/test/factories';

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

describe('courtsApi response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('list resolves with a valid response', async () => {
    mockGet.mockReturnValue(jsonOf({ courts: [{ ...makeCourt(), prices: [makePrice()] }] }));
    const result = await courtsApi.list('c1');
    expect(result.courts).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when prices is missing', async () => {
    mockGet.mockReturnValue(jsonOf({ courts: [makeCourt()] }));
    await expectApiResponseError(courtsApi.list('c1'), 'courtsApi.list');
  });

  it('blockSlot rejects with ApiResponseError carrying its context when blocked_slot is missing', async () => {
    mockPost.mockReturnValue(jsonOf({}));
    await expectApiResponseError(
      courtsApi.blockSlot('c1', 'ct1', {
        date: '2026-03-18',
        start_time: '10:00',
        end_time: '11:00',
      }),
      'courtsApi.blockSlot',
    );
  });
});
