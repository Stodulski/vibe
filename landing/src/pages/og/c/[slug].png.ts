/**
 * GET /og/c/<slug>.png: the share card of a complex, 1200x630 PNG, which WhatsApp, Facebook
 * and the other crawlers show when the complex link is shared. The complex page points its
 * og:image here with a ?v= that changes with the facts the card draws, so the cache never
 * serves an old card for new data. The card is built from the same public API read the complex
 * page uses. The answers and their cache headers live in lib/og/complex-image.ts.
 */
import { PUBLIC_API_URL } from 'astro:env/server';
import { BOOKING_MESSAGES } from '@vibe/booking/messages';
import { publicComplexResponseSchema } from '@vibe/booking/complex-response';
import regularFont from '@fontsource/dm-sans/files/dm-sans-latin-400-normal.woff?inline';
import boldFont from '@fontsource/dm-sans/files/dm-sans-latin-700-normal.woff?inline';
import resvgWasm from '@resvg/resvg-wasm/index_bg.wasm?inline';
import { API_TIMEOUT_MS, ApiError, createPublicApi } from '../../../lib/api.ts';
import { complexShareFacts } from '../../../lib/complex-share.ts';
import { complexImageResponse } from '../../../lib/og/complex-image.ts';
import { renderComplexCard, type CardAssets } from '../../../lib/og/render.ts';

export const prerender = false;

/** Decodes a `data:` URL that Vite inlined at build time into the bytes the renderer reads. */
function bytesOf(dataUrl: string): ArrayBuffer {
  const buffer = Buffer.from(dataUrl.slice(dataUrl.indexOf(',') + 1), 'base64');
  return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);
}

let assets: CardAssets | null = null;

/** The fonts and the wasm, decoded once per function instance. */
function cardAssets(): CardAssets {
  assets ??= { regular: bytesOf(regularFont), bold: bytesOf(boldFont), wasm: bytesOf(resvgWasm) };
  return assets;
}

const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });

export async function GET({ params }: { params: { slug?: string } }): Promise<Response> {
  return complexImageResponse(params.slug ?? '', {
    async readFacts(slug) {
      const parsed = publicComplexResponseSchema.safeParse(await api.complex(slug));
      if (!parsed.success) throw new ApiError('unavailable', 200, { cause: parsed.error });
      return complexShareFacts(parsed.data, BOOKING_MESSAGES.courts.sportTypes);
    },
    render: (facts) => renderComplexCard(facts, cardAssets()),
  });
}
