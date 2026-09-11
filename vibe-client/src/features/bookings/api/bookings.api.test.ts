// @vitest-environment node
const { mockGet, mockPost, mockPut } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, post: mockPost, put: mockPut },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { bookingsApi } from './bookings.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeBooking } from '@/test/factories';

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

describe('bookingsApi response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('list resolves with a valid response', async () => {
    mockGet.mockReturnValue(
      jsonOf({
        bookings: [makeBooking()],
        metadata: { has_more: false },
      }),
    );
    const result = await bookingsApi.list('c1', '2026-03-18');
    expect(result.bookings).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when deposit_amount is a string', async () => {
    mockGet.mockReturnValue(
      jsonOf({
        bookings: [{ ...makeBooking(), deposit_amount: '0' }],
        metadata: { has_more: false },
      }),
    );
    await expectApiResponseError(bookingsApi.list('c1', '2026-03-18'), 'bookingsApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when booking is missing', async () => {
    mockGet.mockReturnValue(jsonOf({}));
    await expectApiResponseError(bookingsApi.getById('c1', 'b1'), 'bookingsApi.getById');
  });
});
