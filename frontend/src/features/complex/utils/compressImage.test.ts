// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { compressImage, LOGO_OPTIONS, COVER_OPTIONS } from './compressImage';

// Mock browser APIs
const mockClose = vi.fn();
const mockDrawImage = vi.fn();
const mockConvertToBlob = vi.fn();

vi.stubGlobal(
  'createImageBitmap',
  vi.fn().mockResolvedValue({
    width: 2000,
    height: 1500,
    close: mockClose,
  }),
);

vi.stubGlobal(
  'OffscreenCanvas',
  class MockOffscreenCanvas {
    width: number;
    height: number;
    constructor(width: number, height: number) {
      this.width = width;
      this.height = height;
    }
    getContext() {
      return { drawImage: mockDrawImage };
    }
    convertToBlob = mockConvertToBlob.mockResolvedValue(new Blob(['test'], { type: 'image/webp' }));
  },
);

describe('compressImage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('scales down image maintaining aspect ratio', async () => {
    const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' });

    await compressImage(file, { maxWidth: 512, maxHeight: 512, quality: 0.8 });

    // 2000x1500 scaled to fit 512x512: ratio = min(512/2000, 512/1500) = 0.256
    // width = 512, height = 384
    expect(mockDrawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, 512, 384);
  });

  it('returns a webp blob', async () => {
    const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' });

    const result = await compressImage(file, { maxWidth: 512, maxHeight: 512, quality: 0.8 });

    expect(result).toBeInstanceOf(Blob);
    expect(mockConvertToBlob).toHaveBeenCalledWith({ type: 'image/webp', quality: 0.8 });
  });

  it('closes the image bitmap after processing', async () => {
    const file = new File(['test'], 'test.jpg', { type: 'image/jpeg' });

    await compressImage(file, { maxWidth: 512, maxHeight: 512, quality: 0.8 });

    expect(mockClose).toHaveBeenCalled();
  });

  it('does not scale up small images', async () => {
    (createImageBitmap as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      width: 200,
      height: 150,
      close: mockClose,
    });

    const file = new File(['test'], 'small.jpg', { type: 'image/jpeg' });

    await compressImage(file, { maxWidth: 512, maxHeight: 512, quality: 0.8 });

    // Image is smaller than max, so dimensions stay as-is
    expect(mockDrawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, 200, 150);
  });

  it('LOGO_OPTIONS has correct dimensions', () => {
    expect(LOGO_OPTIONS.maxWidth).toBe(512);
    expect(LOGO_OPTIONS.maxHeight).toBe(512);
    expect(LOGO_OPTIONS.quality).toBe(0.8);
  });

  it('COVER_OPTIONS has correct dimensions', () => {
    expect(COVER_OPTIONS.maxWidth).toBe(1280);
    expect(COVER_OPTIONS.maxHeight).toBe(720);
    expect(COVER_OPTIONS.quality).toBe(0.8);
  });
});
