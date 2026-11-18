/**
 * GET /kit/c/<slug>/<piece>-<format>.png: one picture of a complex's social media kit, PNG, for
 * the owner to post on their own networks. The piece is reservar or como-reservar, the format
 * historia (1080x1920) or post (1080x1440). ?descargar=1 makes it a download. The answers and
 * their cache headers live in lib/kit/kit-response.ts, and the same public API read as the
 * complex's share card feeds the words.
 */
import { PUBLIC_API_URL } from 'astro:env/server';
import { BOOKING_MESSAGES } from '@vibe/booking/messages';
import { publicComplexResponseSchema } from '@vibe/booking/complex-response';
import regularFont from '@fontsource/dm-sans/files/dm-sans-latin-400-normal.woff?inline';
import boldFont from '@fontsource/dm-sans/files/dm-sans-latin-700-normal.woff?inline';
import resvgWasm from '@resvg/resvg-wasm/index_bg.wasm?inline';
import padelPhoto from '../../../../assets/kit/padel.jpg?inline';
import futbolPhoto from '../../../../assets/kit/futbol.jpg?inline';
import tenisPhoto from '../../../../assets/kit/tenis.jpg?inline';
import vibeLogo from '../../../../../public/logo.svg?inline';
import { API_TIMEOUT_MS, ApiError, createPublicApi } from '../../../../lib/api.ts';
import { complexShareFacts } from '../../../../lib/complex-share.ts';
import { kitImageResponse } from '../../../../lib/kit/kit-response.ts';
import { renderKitImage } from '../../../../lib/kit/render.ts';
import { sharedRead } from '../../../../lib/kit/shared-read.ts';
import type { CardAssets } from '../../../../lib/og/render.ts';

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

/** The backdrop photos, inlined as data URLs at build time (licenses: assets/kit/LICENSES.md). */
const PHOTOS = { padel: padelPhoto, futbol: futbolPhoto, tenis: tenisPhoto } as const;

const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });

/** The four pictures of a kit arrive together, so they share one complex read (see shared-read.ts). */
const readFacts = sharedRead(async (slug: string) => {
  const parsed = publicComplexResponseSchema.safeParse(await api.complex(slug));
  if (!parsed.success) throw new ApiError('unavailable', 200, { cause: parsed.error });
  return complexShareFacts(parsed.data, BOOKING_MESSAGES.courts.sportTypes);
}, 60_000);

export async function GET({ params, url }: { params: { slug?: string; image?: string }; url: URL }): Promise<Response> {
  return kitImageResponse(params.slug ?? '', params.image ?? '', url.searchParams, {
    readFacts,
    render: (facts, slug, image) => renderKitImage(facts, slug, image, cardAssets(), PHOTOS, vibeLogo),
  });
}
