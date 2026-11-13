// @vitest-environment node
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { cropImage, cropDrawArgs, COVER_TARGET, LOGO_TARGET } from './cropImage';

const mockClose = vi.fn();
const mockDrawImage = vi.fn();
const mockConvertToBlob = vi.fn();
let canvasSize: [number, number] | null = null;

vi.stubGlobal('createImageBitmap', vi.fn().mockResolvedValue({ width: 2000, height: 1500, close: mockClose }));
vi.stubGlobal(
  'OffscreenCanvas',
  class MockOffscreenCanvas {
    constructor(width: number, height: number) {
      canvasSize = [width, height];
    }
    getContext() {
      return { drawImage: mockDrawImage };
    }
    convertToBlob = mockConvertToBlob;
  },
);

const AREA = { x: 100, y: 50, width: 1600, height: 900 };

describe('cropImage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    canvasSize = null;
    mockConvertToBlob.mockResolvedValue(new Blob(['x'], { type: 'image/webp' }));
  });

  it('maps the pixel area onto a canvas of exactly the target size', () => {
    expect(cropDrawArgs(AREA, COVER_TARGET)).toEqual([100, 50, 1600, 900, 0, 0, 1280, 720]);
    expect(cropDrawArgs({ x: 0, y: 0, width: 300, height: 300 }, LOGO_TARGET)).toEqual([
      0, 0, 300, 300, 0, 0, 512, 512,
    ]);
  });

  it('draws the area, encodes WebP and closes the bitmap', async () => {
    const file = new File(['x'], 'a.jpg', { type: 'image/jpeg' });

    const blob = await cropImage(file, AREA, COVER_TARGET);

    expect(canvasSize).toEqual([1280, 720]);
    expect(mockDrawImage).toHaveBeenCalledWith(expect.anything(), 100, 50, 1600, 900, 0, 0, 1280, 720);
    expect(mockConvertToBlob).toHaveBeenCalledWith({ type: 'image/webp', quality: 0.8 });
    expect(blob).toBeInstanceOf(Blob);
    expect(mockClose).toHaveBeenCalled();
  });

  it('closes the bitmap even when encoding fails', async () => {
    mockConvertToBlob.mockRejectedValueOnce(new Error('boom'));
    const file = new File(['x'], 'a.jpg', { type: 'image/jpeg' });

    await expect(cropImage(file, AREA, LOGO_TARGET)).rejects.toThrow('boom');
    expect(mockClose).toHaveBeenCalled();
  });
});
