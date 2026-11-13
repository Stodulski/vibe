/** A crop rectangle in the source image's own pixels (react-easy-crop's `croppedAreaPixels`). */
export interface CropArea {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface CropTarget {
  width: number;
  height: number;
  quality: number;
}

/** The exact size each kind of upload is stored at. */
export const LOGO_TARGET: CropTarget = { width: 512, height: 512, quality: 0.8 };
export const COVER_TARGET: CropTarget = { width: 1280, height: 720, quality: 0.8 };

/** `drawImage(source, sx, sy, sw, sh, dx, dy, dw, dh)` arguments after the source. */
export function cropDrawArgs(area: CropArea, target: CropTarget) {
  return [area.x, area.y, area.width, area.height, 0, 0, target.width, target.height] as const;
}

/** Draws the chosen area of `file` onto a canvas of exactly `target` size and encodes it as WebP. */
export async function cropImage(file: File, area: CropArea, target: CropTarget): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  try {
    const canvas = new OffscreenCanvas(target.width, target.height);
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('2d canvas context is not supported in this browser');
    }
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(bitmap, ...cropDrawArgs(area, target));
    return await canvas.convertToBlob({ type: 'image/webp', quality: target.quality });
  } finally {
    bitmap.close();
  }
}
