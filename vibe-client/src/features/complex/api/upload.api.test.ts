// @vitest-environment node
const { mockPost } = vi.hoisted(() => ({ mockPost: vi.fn() }));

vi.mock('@/shared/lib/ky', () => ({
  default: { post: mockPost, delete: vi.fn() },
}));

import { uploadApi } from './upload.api';
import { ApiResponseError } from '@/shared/lib/apiParse';

function jsonOf(body: unknown) {
  return { json: vi.fn().mockResolvedValue(body) };
}

describe('uploadApi response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('presign resolves with a valid response', async () => {
    mockPost.mockReturnValue(
      jsonOf({
        upload_url: 'https://r2.test/put',
        public_url: 'https://cdn.test/logo.webp',
        key: 'k1',
      }),
    );
    const result = await uploadApi.presign('c1', {
      type: 'logo',
      content_type: 'image/webp',
      file_size: 100,
    });
    expect(result.key).toBe('k1');
  });

  it('presign rejects with ApiResponseError carrying its context when upload_url is missing', async () => {
    mockPost.mockReturnValue(jsonOf({ public_url: 'https://cdn.test/logo.webp', key: 'k1' }));
    try {
      await uploadApi.presign('c1', { type: 'logo', content_type: 'image/webp', file_size: 100 });
      throw new Error('expected promise to reject');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiResponseError);
      expect((err as ApiResponseError).context).toBe('uploadApi.presign');
    }
  });
});
