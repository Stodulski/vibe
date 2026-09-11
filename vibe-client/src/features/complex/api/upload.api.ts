import { z } from 'zod';
import api from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';

export interface PresignRequest {
  type: 'logo' | 'cover';
  content_type: string;
  file_size: number;
}

export interface PresignResponse {
  upload_url: string;
  public_url: string;
  key: string;
}

// Declared locally (not in `src/shared/types/api.types/`) — this endpoint's
// contract lives entirely in this file, so its response schema does too.
const presignResponseSchema = z
  .object({
    upload_url: z.string(),
    public_url: z.string(),
    key: z.string(),
  })
  .loose() satisfies z.ZodType<PresignResponse>;

export const uploadApi = {
  presign: (complexId: string, data: PresignRequest): Promise<PresignResponse> =>
    api
      .post(`complexes/${complexId}/uploads/presign`, { json: data })
      .json()
      .then(parseWith(presignResponseSchema, 'uploadApi.presign')),

  deleteImage: (complexId: string, url: string) =>
    api.delete(`complexes/${complexId}/uploads`, { json: { url } }).json(),

  uploadToR2: (presignedUrl: string, file: Blob) =>
    fetch(presignedUrl, {
      method: 'PUT',
      headers: { 'Content-Type': 'image/webp' },
      body: file,
    }),
};
