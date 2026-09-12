import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useUploadImage, useDeleteImage } from './useUploadImage';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';

const t = ES_AR;

const mockPresign = vi.fn();
const mockUploadToR2 = vi.fn();
const mockDeleteImage = vi.fn();
const mockUpdate = vi.fn();

vi.mock('../api/upload.api', () => ({
  uploadApi: {
    presign: (...args: unknown[]) => mockPresign(...args) as unknown,
    uploadToR2: (...args: unknown[]) => mockUploadToR2(...args) as unknown,
    deleteImage: (...args: unknown[]) => mockDeleteImage(...args) as unknown,
  },
}));

vi.mock('../api/complex.api', () => ({
  complexApi: {
    update: (...args: unknown[]) => mockUpdate(...args) as unknown,
  },
}));

vi.mock('../utils/compressImage', () => ({
  compressImage: vi.fn().mockResolvedValue(new Blob(['x'], { type: 'image/webp' })),
  LOGO_OPTIONS: {},
  COVER_OPTIONS: {},
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@sentry/react', () => ({ captureException: vi.fn() }));

describe('useUploadImage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPresign.mockResolvedValue({
      upload_url: 'https://r2.test/put',
      public_url: 'https://cdn.test/logo.webp',
      key: 'k1',
    });
    mockUpdate.mockResolvedValue({ id: 'c1' });
  });

  // Finding M13: a non-OK R2 response used to throw a hardcoded English
  // `Error('Upload failed')`, which `onError` then showed verbatim to an
  // Argentine Spanish UI via `error.message || fallback` (message wins
  // because it's truthy).
  it('reports the i18n upload error, not the hardcoded English message, when the R2 PUT fails', async () => {
    mockUploadToR2.mockResolvedValue({ ok: false });
    const { toast } = await import('sonner');

    const { result } = renderHook(() => useUploadImage('c1'), { wrapper: createQueryWrapper() });
    result.current.mutate({
      file: new File(['x'], 'logo.png', { type: 'image/png' }),
      type: 'logo',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(t.complex.imageUploadError);
    expect(toast.error).not.toHaveBeenCalledWith('Upload failed');
  });

  // ERR-02: a raw, untranslated error from outside the app's own validation
  // (here standing in for `compressImage`'s `createImageBitmap` throwing a
  // browser-internal DOMException) must not reach the toast verbatim.
  it('reports the generic i18n upload error, not a raw technical message, for an unexpected failure', async () => {
    const { compressImage } = await import('../utils/compressImage');
    vi.mocked(compressImage).mockRejectedValueOnce(new Error('The source image could not be decoded'));
    const { toast } = await import('sonner');

    const { result } = renderHook(() => useUploadImage('c1'), { wrapper: createQueryWrapper() });
    result.current.mutate({
      file: new File(['x'], 'logo.png', { type: 'image/png' }),
      type: 'logo',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(t.complex.imageUploadError);
    expect(toast.error).not.toHaveBeenCalledWith('The source image could not be decoded');
  });

  // Its own validation message stays specific and in Spanish, unlike the
  // generic fallback above — this is the one case where showing `error.message`
  // as-is is correct, since the app itself wrote it.
  it('reports the specific "invalid type" message for a rejected file type', async () => {
    const { toast } = await import('sonner');

    const { result } = renderHook(() => useUploadImage('c1'), { wrapper: createQueryWrapper() });
    result.current.mutate({
      file: new File(['x'], 'logo.gif', { type: 'image/gif' }),
      type: 'logo',
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(t.complex.imageInvalidType);
  });
});

describe('useDeleteImage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdate.mockResolvedValue({ id: 'c1' });
  });

  // Finding M13: the R2 cleanup after clearing a complex's image was wrapped
  // in `catch { /* Non-critical */ }` — a real failure (leaving an orphaned
  // file in R2) disappeared with no trace anywhere.
  it('reports a failed best-effort R2 cleanup to Sentry instead of swallowing it', async () => {
    const cleanupError = new Error('R2 delete failed');
    mockDeleteImage.mockRejectedValue(cleanupError);
    const Sentry = await import('@sentry/react');
    const { toast } = await import('sonner');

    const { result } = renderHook(() => useDeleteImage('c1'), { wrapper: createQueryWrapper() });
    result.current.mutate({ type: 'logo', currentUrl: 'https://cdn.test/logo.webp' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(Sentry.captureException).toHaveBeenCalledWith(cleanupError);
    // Still a non-critical failure: the complex update itself succeeded.
    expect(toast.success).toHaveBeenCalledWith(t.complex.imageDeleted);
  });
});
