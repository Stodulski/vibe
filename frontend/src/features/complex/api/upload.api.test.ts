import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { uploadApi } from './upload.api';
import { ApiResponseError } from '@/shared/lib/apiParse';

describe('uploadApi response validation', () => {
  it('presign resolves with a valid response', async () => {
    server.use(
      http.post('*/complexes/:complexId/uploads/presign', () =>
        HttpResponse.json({
          upload_url: 'https://r2.test/put',
          public_url: 'https://cdn.test/logo.webp',
          key: 'k1',
        }),
      ),
    );
    const result = await uploadApi.presign('c1', {
      type: 'logo',
      content_type: 'image/webp',
      file_size: 100,
    });
    expect(result.key).toBe('k1');
  });

  it('presign rejects with ApiResponseError carrying its context when upload_url is missing', async () => {
    server.use(
      http.post('*/complexes/:complexId/uploads/presign', () =>
        HttpResponse.json({ public_url: 'https://cdn.test/logo.webp', key: 'k1' }),
      ),
    );
    try {
      await uploadApi.presign('c1', { type: 'logo', content_type: 'image/webp', file_size: 100 });
      throw new Error('expected promise to reject');
    } catch (err) {
      expect(err).toBeInstanceOf(ApiResponseError);
      expect((err as ApiResponseError).context).toBe('uploadApi.presign');
    }
  });
});
