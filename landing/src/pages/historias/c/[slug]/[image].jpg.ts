/**
 * GET /historias/c/<slug>/<piece>.jpg: an Instagram story a complex publishes, 1080x1920 JPEG.
 * libres lists the day's free hours (?fecha=YYYY-MM-DD, today by default); liberado announces one
 * freed slot (?hora=HH:MM&cancha=<court id>, plus ?fecha). Both are drawn from the public
 * availability grid only, so the picture never shows an hour that cannot be booked now. The
 * answers and their cache headers live in lib/story/story-response.ts. No QR, only the address.
 */
import { PUBLIC_API_URL } from 'astro:env/server';
import { BOOKING_MESSAGES } from '@vibe/booking/messages';
import { publicComplexResponseSchema } from '@vibe/booking/complex-response';
import { availabilityEnvelopeSchema } from '@vibe/booking/availability-response';
import { venueDateString } from '@vibe/booking/venue-date';
import regularFont from '@fontsource/dm-sans/files/dm-sans-latin-400-normal.woff?inline';
import boldFont from '@fontsource/dm-sans/files/dm-sans-latin-700-normal.woff?inline';
import resvgWasm from '@resvg/resvg-wasm/index_bg.wasm?inline';
import padelPhoto from '../../../../assets/kit/padel.jpg?inline';
import futbolPhoto from '../../../../assets/kit/futbol.jpg?inline';
import tenisPhoto from '../../../../assets/kit/tenis.jpg?inline';
import vibeLogo from '../../../../../public/logo.svg?inline';
import { API_TIMEOUT_MS, ApiError, createPublicApi } from '../../../../lib/api.ts';
import { complexShareFacts } from '../../../../lib/complex-share.ts';
import { kitBackdrop } from '../../../../lib/kit/backdrop.ts';
import { rasterizeJpeg, type CardAssets } from '../../../../lib/og/render.ts';
import { storyImageResponse } from '../../../../lib/story/story-response.ts';
import { STORY_HEIGHT, STORY_WIDTH, storyTree } from '../../../../lib/story/story-tree.ts';

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

const PHOTOS = { padel: padelPhoto, futbol: futbolPhoto, tenis: tenisPhoto } as const;
const LABELS = BOOKING_MESSAGES.courts.sportTypes;

const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });

export async function GET({ params, url }: { params: { slug?: string; image?: string }; url: URL }): Promise<Response> {
  return storyImageResponse(params.slug ?? '', params.image ?? '', url.searchParams, {
    today: venueDateString(),
    labels: LABELS,
    async readFacts(slug) {
      const parsed = publicComplexResponseSchema.safeParse(await api.complex(slug));
      if (!parsed.success) throw new ApiError('unavailable', 200, { cause: parsed.error });
      return complexShareFacts(parsed.data, LABELS);
    },
    async readAvailability(slug, date, duration) {
      const parsed = availabilityEnvelopeSchema.safeParse(await api.availability(slug, date, duration));
      if (!parsed.success) throw new ApiError('unavailable', 200, { cause: parsed.error });
      return parsed.data.availability;
    },
    render(text, facts) {
      const art = { photo: PHOTOS[kitBackdrop(facts.sports)], logo: vibeLogo };
      return rasterizeJpeg(storyTree(text, art), STORY_WIDTH, STORY_HEIGHT, cardAssets());
    },
  });
}
