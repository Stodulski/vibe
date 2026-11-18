/**
 * The layout of the kit pictures: the React tree satori lays out. A piece is a column of blocks,
 * centered inside the layout's margins, on the card palette of og/render.ts. The key text never
 * leaves the margins, so a story's top and bottom bars cover none of it.
 */
import { createElement, type CSSProperties, type ReactElement, type ReactNode } from 'react';
import { BG, FONT_FAMILY, GREEN, MUTED, TEXT, box, glow } from '../og/render.ts';
import type { KitImage, KitLayout } from './kit-image.ts';
import type { KitText } from './kit-text.ts';

/** The white tile the QR sits on. A scanner reads dark modules on a light ground. */
const TILE = '#ffffff';

const CENTERED: CSSProperties = { flexDirection: 'column', alignItems: 'center' };

/** Lifts the words off the photo. */
const SHADOW = '0 4px 24px rgba(0,0,0,0.6)';

/** The glass the steps sit on: light enough to read the photo through, dark enough for the words. */
const GLASS: CSSProperties = {
  backgroundColor: 'rgba(11,11,11,0.55)',
  border: '2px solid rgba(255,255,255,0.12)',
  borderRadius: 32,
};

/**
 * The tint over the photo: black from the bottom, where most words sit, and green from the top
 * corner, the brand's light. The photo stays visible in the middle band.
 */
const OVERLAY_DARK =
  'linear-gradient(180deg, rgba(11,11,11,0.3) 0%, rgba(11,11,11,0.1) 35%, rgba(11,11,11,0.55) 70%, rgba(11,11,11,0.8) 100%)';
const OVERLAY_GREEN = 'linear-gradient(150deg, rgba(29,185,84,0.4) 0%, rgba(29,185,84,0.08) 40%, rgba(29,185,84,0) 60%)';

/** The complex name, clamped to two lines, and the headline under it. */
function titleBlock(text: KitText, layout: KitLayout): ReactElement {
  return box(
    { ...CENTERED, width: '100%', gap: 24 },
    box(
      {
        display: '-webkit-box',
        width: '100%',
        fontSize: layout.nameSize,
        fontWeight: 700,
        lineHeight: 1.05,
        letterSpacing: -2,
        textAlign: 'center',
        overflow: 'hidden',
        WebkitLineClamp: 2,
        WebkitBoxOrient: 'vertical',
        textOverflow: 'ellipsis',
        textShadow: SHADOW,
      },
      text.name,
    ),
    box({ fontSize: 72, fontWeight: 700, color: GREEN, textAlign: 'center', letterSpacing: -1, textShadow: SHADOW }, text.headline),
  );
}

/**
 * The QR picture on its white tile. The picture carries its own quiet zone (4 modules, the
 * standard), so the tile adds only a thin rim inside its green border.
 */
function qrTile(src: string, size: number): ReactElement {
  const padding = 8;
  const picture = size - padding * 2;
  return box(
    { position: 'relative', width: size, height: size },
    qrGlow(size),
    box(
      {
        // Positioned, so the tile paints above the absolute glow drawn before it.
        position: 'relative',
        width: size,
        height: size,
        padding,
        borderRadius: 40,
        backgroundColor: TILE,
        border: `8px solid ${GREEN}`,
        alignItems: 'center',
        justifyContent: 'center',
      },
      createElement('img', { src, width: picture, height: picture, style: { width: picture, height: picture } }),
    ),
  );
}

/** How far the green light reaches past the QR tile. */
const QR_GLOW_REACH = 100;

/**
 * The green light around the QR tile, drawn as a gradient rather than a box-shadow: resvg blurs a
 * shadow this wide in about seven seconds, and the picture has to render well inside the time an
 * email client waits for an image.
 */
function qrGlow(size: number): ReactElement {
  const span = size + QR_GLOW_REACH * 2;
  const edge = Math.round((size / 2 / (span / 2)) * 100);
  return box({
    position: 'absolute',
    top: -QR_GLOW_REACH,
    left: -QR_GLOW_REACH,
    width: span,
    height: span,
    borderRadius: span / 2,
    backgroundImage: `radial-gradient(circle closest-side, rgba(29,185,84,0.45) ${edge}%, rgba(29,185,84,0) 100%)`,
  });
}

/** Under the QR of reservar: the scan line, the link in bold, and the sports line when there is one. */
function reservarLines(text: KitText): ReactElement {
  return box(
    { ...CENTERED, gap: 12 },
    box({ fontSize: 44, color: MUTED, textAlign: 'center', textShadow: SHADOW }, text.scanLine),
    box({ fontSize: 52, fontWeight: 700, textAlign: 'center', textShadow: SHADOW }, text.link),
    ...(text.sports ? [box({ fontSize: 34, color: MUTED, textAlign: 'center', textShadow: SHADOW }, text.sports)] : []),
  );
}

/** One step: a green circle with its number, and the words beside it. */
function stepRow(step: string, index: number, fontSize: number): ReactElement {
  return box(
    { flexDirection: 'row', alignItems: 'center', gap: 32, width: '100%' },
    box(
      {
        width: 96,
        height: 96,
        flexShrink: 0,
        borderRadius: 48,
        backgroundColor: GREEN,
        color: BG,
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 48,
        fontWeight: 700,
      },
      String(index + 1),
    ),
    box({ flexGrow: 1, fontSize, lineHeight: 1.2 }, step),
  );
}

/** The link under the smaller QR of como-reservar. */
function linkLine(link: string): ReactElement {
  return box({ fontSize: 40, fontWeight: 700, textAlign: 'center', textShadow: SHADOW }, link);
}

/** The width the Vibe logo is drawn at; its height follows the logo's own proportions. */
const LOGO_WIDTH = 220;
/** logo.svg's viewBox is 975.07 x 683.53. */
const LOGO_HEIGHT = Math.round((LOGO_WIDTH * 683.53) / 975.07);

/** The Vibe logo, the same mark the landing shows. */
function brandMark(logo: string): ReactElement {
  return createElement('img', {
    src: logo,
    width: LOGO_WIDTH,
    height: LOGO_HEIGHT,
    style: { width: LOGO_WIDTH, height: LOGO_HEIGHT },
  });
}

function reservarBlocks(text: KitText, qrSrc: string, layout: KitLayout, art: KitArt): ReactNode[] {
  return [titleBlock(text, layout), qrTile(qrSrc, layout.qrSize), reservarLines(text), brandMark(art.logo)];
}

function comoReservarBlocks(text: KitText, qrSrc: string, layout: KitLayout, art: KitArt): ReactNode[] {
  return [
    titleBlock(text, layout),
    box(
      { ...GLASS, flexDirection: 'column', gap: 36, width: '100%', padding: '40px 36px' },
      ...text.steps.map((step, index) => stepRow(step, index, layout.stepSize)),
    ),
    box({ ...CENTERED, gap: 24 }, qrTile(qrSrc, layout.qrSmallSize), linkLine(text.link)),
    brandMark(art.logo),
  ];
}

/** The photo filling the picture, cropped from its center, under the black and green tint. */
function backdrop(photo: string, layout: KitLayout): ReactNode[] {
  const fill: CSSProperties = { position: 'absolute', top: 0, left: 0, width: layout.width, height: layout.height };
  return [
    createElement('img', {
      src: photo,
      width: layout.width,
      height: layout.height,
      style: { ...fill, objectFit: 'cover' },
    }),
    box({ ...fill, backgroundImage: OVERLAY_DARK }),
    box({ ...fill, backgroundImage: OVERLAY_GREEN }),
  ];
}

/** The picture's frame: the photo and its tint, the glow and the column inside the margins. */
function frame(layout: KitLayout, photo: string, children: ReactNode[]): ReactElement {
  return box(
    {
      position: 'relative',
      width: layout.width,
      height: layout.height,
      flexDirection: 'column',
      overflow: 'hidden',
      backgroundColor: BG,
      color: TEXT,
      fontFamily: FONT_FAMILY,
    },
    ...backdrop(photo, layout),
    glow({ top: -200, right: -260 }, 900),
    box(
      {
        position: 'absolute',
        top: layout.top,
        right: layout.side,
        bottom: layout.bottom,
        left: layout.side,
        ...CENTERED,
        justifyContent: 'space-between',
        gap: layout.gap,
      },
      ...children,
    ),
  );
}

/** The pictures a kit image draws, as data URLs: the backdrop photo and the Vibe logo. */
export interface KitArt {
  photo: string;
  logo: string;
}

export function kitTree(text: KitText, qrSrc: string, image: KitImage, layout: KitLayout, art: KitArt): ReactElement {
  const blocks =
    image.piece === 'reservar' ? reservarBlocks(text, qrSrc, layout, art) : comoReservarBlocks(text, qrSrc, layout, art);
  return frame(layout, art.photo, blocks);
}
