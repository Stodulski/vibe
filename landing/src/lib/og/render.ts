/**
 * Draws a complex's share card: satori lays the words out as SVG, and resvg-wasm turns the
 * SVG into a 1200x630 PNG. The fonts and the wasm arrive as bytes from the caller, so this
 * module reads no file and runs the same under node:test and in the Vercel function.
 */
import { createElement, type CSSProperties, type ReactElement, type ReactNode } from 'react';
import satori from 'satori';
import { initWasm, Resvg, type InitInput } from '@resvg/resvg-wasm';
import { complexCardText, type ComplexCardText } from './complex-card.ts';
import { SHARE_IMAGE_HEIGHT, SHARE_IMAGE_WIDTH, type ShareFacts } from '../complex-share.ts';

/** The landing's dark tokens (packages/ui tokens.css and the booking palette). */
const BG = '#0b0b0b';
const GREEN = '#1db954';
const TEXT = '#ffffff';
const MUTED = '#b0b0b0';
const TERTIARY = '#8a8a8a';

const FONT_FAMILY = 'DM Sans';

export interface CardAssets {
  /** DM Sans 400 and 700 in WOFF or TTF: satori reads neither WOFF2 nor variable axes. */
  regular: ArrayBuffer;
  bold: ArrayBuffer;
  /** The resvg wasm module, as bytes or a compiled module. */
  wasm: InitInput;
}

/** A flex box, the only element the card uses. satori reads React elements, so this is createElement. */
function box(style: CSSProperties, ...children: ReactNode[]): ReactElement {
  return createElement('div', { style: { display: 'flex', ...style } }, ...children);
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
      box({
        position: 'absolute',
        top: -260,
        right: -240,
        width: 820,
        height: 820,
        borderRadius: 410,
        backgroundImage: `radial-gradient(circle, rgba(29,185,84,0.22) 0%, rgba(29,185,84,0) 62%)`,
      }),
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

export async function renderComplexCard(facts: ShareFacts, assets: CardAssets): Promise<ArrayBuffer> {
  await ensureResvg(assets.wasm);
  const svg = await satori(cardTree(complexCardText(facts)), {
    width: SHARE_IMAGE_WIDTH,
    height: SHARE_IMAGE_HEIGHT,
    fonts: [
      { name: FONT_FAMILY, data: assets.regular, weight: 400, style: 'normal' },
      { name: FONT_FAMILY, data: assets.bold, weight: 700, style: 'normal' },
    ],
  });
  const resvg = new Resvg(svg, { fitTo: { mode: 'width', value: SHARE_IMAGE_WIDTH } });
  try {
    const png = resvg.render().asPng();
    return png.buffer.slice(png.byteOffset, png.byteOffset + png.byteLength) as ArrayBuffer;
  } finally {
    resvg.free();
  }
}
