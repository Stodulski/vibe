interface CompressOptions {
  maxWidth: number;
  maxHeight: number;
  quality: number;
}

export async function compressImage(file: File, options: CompressOptions): Promise<Blob> {
  const { maxWidth, maxHeight, quality } = options;

  const bitmap = await createImageBitmap(file);

  let width = bitmap.width;
  let height = bitmap.height;

  // Scale down maintaining aspect ratio.
  if (width > maxWidth || height > maxHeight) {
    const ratio = Math.min(maxWidth / width, maxHeight / height);
    width = Math.round(width * ratio);
    height = Math.round(height * ratio);
  }

  const canvas = new OffscreenCanvas(width, height);
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    throw new Error('2d canvas context is not supported in this browser');
  }
  ctx.drawImage(bitmap, 0, 0, width, height);
  bitmap.close();

  const blob = await canvas.convertToBlob({ type: 'image/webp', quality });
  return blob;
}

export const LOGO_OPTIONS: CompressOptions = { maxWidth: 512, maxHeight: 512, quality: 0.8 };
export const COVER_OPTIONS: CompressOptions = { maxWidth: 1280, maxHeight: 720, quality: 0.8 };
