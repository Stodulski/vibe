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

import { complexApi } from './complex.api';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { makeComplex } from '@/test/factories';

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

describe('complexApi response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('list resolves with a valid response', async () => {
    mockGet.mockReturnValue(jsonOf({ complexes: [makeComplex()] }));
    const result = await complexApi.list();
    expect(result.complexes).toHaveLength(1);
  });

  it('list rejects with ApiResponseError carrying its context when is_active is a string', async () => {
    mockGet.mockReturnValue(jsonOf({ complexes: [{ ...makeComplex(), is_active: 'yes' }] }));
    await expectApiResponseError(complexApi.list(), 'complexApi.list');
  });

  it('getById rejects with ApiResponseError carrying its context when complex is missing', async () => {
    mockGet.mockReturnValue(jsonOf({}));
    await expectApiResponseError(complexApi.getById('c1'), 'complexApi.getById');
  });

  it('delete rejects with ApiResponseError carrying its context when courts_deactivated is missing', async () => {
    mockDelete.mockReturnValue(jsonOf({ message: 'ok' }));
    await expectApiResponseError(complexApi.delete('c1'), 'complexApi.delete');
  });

  it('connectMP posts the OAuth exchange to the complex MP endpoint and resolves with a valid response', async () => {
    mockPost.mockReturnValue(jsonOf({ connected: true, mp_user_id: 'mp1' }));
    const result = await complexApi.connectMP('c1', {
      code: 'oauth-code',
      redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
      code_verifier: 'pkce-verifier',
    });
    expect(mockPost).toHaveBeenCalledWith('complexes/c1/mp/connect', {
      json: {
        code: 'oauth-code',
        redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
        code_verifier: 'pkce-verifier',
      },
    });
    expect(result.connected).toBe(true);
  });

  it('connectMP omits code_verifier when the authorize step ran without PKCE', async () => {
    mockPost.mockReturnValue(jsonOf({ connected: true }));
    await complexApi.connectMP('c1', {
      code: 'oauth-code',
      redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback',
    });
    expect(mockPost).toHaveBeenCalledWith('complexes/c1/mp/connect', {
      json: { code: 'oauth-code', redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback' },
    });
  });

  it('connectMP rejects with ApiResponseError carrying its context when connected is not a boolean', async () => {
    mockPost.mockReturnValue(jsonOf({ connected: 'yes' }));
    await expectApiResponseError(
      complexApi.connectMP('c1', { code: 'oauth-code', redirect_uri: 'https://app.vibe.com.ar/settings/mp/callback' }),
      'complexApi.connectMP',
    );
  });
});
