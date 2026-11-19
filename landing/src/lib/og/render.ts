/**
 * Draws a complex's share card: satori lays the words out as SVG, and resvg-wasm turns the
 * SVG into a 1200x630 PNG. The fonts and the wasm arrive as bytes from the caller, so this
 * module reads no file and runs the same under node:test and in the Vercel function.
 */
import { createElement, type CSSProperties, type ReactElement, type ReactNode } from 'react';
import satori from 'satori';
import { initWasm, Resvg, type InitInput } from '@resvg/resvg-wasm';
import jpeg from 'jpeg-js';
import { complexCardText, type ComplexCardText } from './complex-card.ts';
import { SHARE_IMAGE_HEIGHT, SHARE_IMAGE_WIDTH, type ShareFacts } from '../complex-share.ts';

/** The landing's dark tokens (packages/ui tokens.css and the booking palette). */
export const BG = '#0b0b0b';
export const GREEN = '#1db954';
export const TEXT = '#ffffff';
export const MUTED = '#b0b0b0';
export const TERTIARY = '#8a8a8a';

/** The green light behind every card: a radial glow that fades out before the edge of its box. */
const GLOW = 'radial-gradient(circle, rgba(29,185,84,0.22) 0%, rgba(29,185,84,0) 62%)';

export const FONT_FAMILY = 'DM Sans';

export interface CardAssets {
  /** DM Sans 400 and 700 in WOFF or TTF: satori reads neither WOFF2 nor variable axes. */
  regular: ArrayBuffer;
  bold: ArrayBuffer;
  /** The resvg wasm module, as bytes or a compiled module. */
  wasm: InitInput;
}

/** A flex box, the only element the cards use. satori reads React elements, so this is createElement. */
export function box(style: CSSProperties, ...children: ReactNode[]): ReactElement {
  return createElement('div', { style: { display: 'flex', ...style } }, ...children);
}

/** The green glow of a card, a circle of the given size placed by its position styles. */
export function glow(placement: CSSProperties, size: number): ReactElement {
  return box({
    position: 'absolute',
    ...placement,
    width: size,
    height: size,
    borderRadius: size / 2,
    backgroundImage: GLOW,
  });
}

/** The card's tree. Text lines that are empty are left out, so no empty box is drawn. */
function cardTree(text: ComplexCardText): ReactElement {
  return box(
    {
      position: 'relative',
      width: SHARE_IMAGE_WIDTH,
      height: SHARE_IMAGE_HEIGHT,
      flexDirection: 'column',
      justifyContent: 'space-between',
      overflow: 'hidden',
      padding: '64px 72px',
      backgroundColor: BG,
      color: TEXT,
      fontFamily: FONT_FAMILY,
    },
    [
      glow({ top: -260, right: -240 }, 820),
      box({ fontSize: 40, fontWeight: 700, color: GREEN, letterSpacing: -1 }, 'Vibe'),
      box({ flexDirection: 'column', gap: 16 }, [
        box(
          {
            display: '-webkit-box',
            maxWidth: 1056,
            fontSize: 88,
            fontWeight: 700,
            lineHeight: 1.05,
            letterSpacing: -2,
            overflow: 'hidden',
            WebkitLineClamp: 2,
            WebkitBoxOrient: 'vertical',
            textOverflow: 'ellipsis',
          },
          text.name,
        ),
        ...(text.place ? [box({ fontSize: 40, color: MUTED }, text.place)] : []),
        ...(text.sports ? [box({ fontSize: 40, fontWeight: 700, color: GREEN }, text.sports)] : []),
      ]),
      box({ fontSize: 28, color: TERTIARY }, 'vibe.com.ar'),
    ],
  );
}

let wasmReady: Promise<void> | null = null;

/** Initializes resvg once per isolate. A failed start is forgotten, so the next request tries again. */
function ensureResvg(wasm: InitInput): Promise<void> {
  wasmReady ??= initWasm(wasm).catch((error: unknown) => {
    wasmReady = null;
    throw error;
  });
  return wasmReady;
}

/**
 * Lays a tree out as SVG with satori, then rasterizes it with resvg at its own width, or scaled to
 * outputWidth when one is given. Fewer pixels render faster.
 */
export async function rasterize(
  tree: ReactElement,
  width: number,
  height: number,
  assets: CardAssets,
  outputWidth: number = width,
): Promise<ArrayBuffer> {
  return withRender(tree, width, height, assets, outputWidth, (rendered) => {
    const png = rendered.asPng();
    return png.buffer.slice(png.byteOffset, png.byteOffset + png.byteLength) as ArrayBuffer;
  });
}

/** JPEG quality for the stories: Instagram takes only JPEG for a story, and 88 keeps the photo clean. */
const JPEG_QUALITY = 88;

/** The same as rasterize, encoded as JPEG: Instagram publishes a story only from a JPEG. */
export async function rasterizeJpeg(
  tree: ReactElement,
  width: number,
  height: number,
  assets: CardAssets,
): Promise<ArrayBuffer> {
  return withRender(tree, width, height, assets, width, (rendered) => {
    const encoded = jpeg.encode(
      { data: rendered.pixels, width: rendered.width, height: rendered.height },
      JPEG_QUALITY,
    ).data;
    return encoded.buffer.slice(encoded.byteOffset, encoded.byteOffset + encoded.byteLength) as ArrayBuffer;
  });
}

type Rendered = ReturnType<InstanceType<typeof Resvg>['render']>;

/** Lays the tree out with satori, rasterizes it with resvg, and hands the pixels to encode. */
async function withRender(
  tree: ReactElement,
  width: number,
  height: number,
  assets: CardAssets,
  outputWidth: number,
  encode: (rendered: Rendered) => ArrayBuffer,
): Promise<ArrayBuffer> {
  await ensureResvg(assets.wasm);
  const svg = await satori(tree, {
    width,
    height,
    fonts: [
      { name: FONT_FAMILY, data: assets.regular, weight: 400, style: 'normal' },
      { name: FONT_FAMILY, data: assets.bold, weight: 700, style: 'normal' },
    ],
  });
  const resvg = new Resvg(svg, { fitTo: { mode: 'width', value: outputWidth } });
  try {
    const rendered = resvg.render();
    try {
      return encode(rendered);
    } finally {
      rendered.free();
    }
  } finally {
    resvg.free();
  }
}

export function renderComplexCard(facts: ShareFacts, assets: CardAssets): Promise<ArrayBuffer> {
  return rasterize(cardTree(complexCardText(facts)), SHARE_IMAGE_WIDTH, SHARE_IMAGE_HEIGHT, assets);
}
