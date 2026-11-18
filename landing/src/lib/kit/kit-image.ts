/**
 * The kit's pictures: the two pieces, the two formats, the layout each format draws with, and
 * the names a picture is served and downloaded under. Pure, so node:test can run it.
 */

export const KIT_PIECES = ['reservar', 'como-reservar'] as const;
export type KitPiece = (typeof KIT_PIECES)[number];

export const KIT_FORMATS = ['historia', 'post'] as const;
export type KitFormat = (typeof KIT_FORMATS)[number];

export interface KitImage {
  piece: KitPiece;
  format: KitFormat;
}

export interface KitLayout {
  width: number;
  height: number;
  /** Clear space above the first key line. A story keeps 220px free for the platform's top bar. */
  top: number;
  /** Clear space below the last key line. A story keeps 260px free for the reply bar. */
  bottom: number;
  /** The margin on both sides. */
  side: number;
  /** Space between the blocks of a piece. */
  gap: number;
  /** Side of the QR of reservar. */
  qrSize: number;
  /** Side of the QR of como-reservar, which sits under the steps. */
  qrSmallSize: number;
  /** Font size of the complex name. */
  nameSize: number;
  /** Font size of the three steps of como-reservar. */
  stepSize: number;
}

const LAYOUTS: Record<KitFormat, KitLayout> = {
  historia: {
    width: 1080,
    height: 1920,
    top: 220,
    bottom: 260,
    side: 64,
    gap: 56,
    qrSize: 560,
    qrSmallSize: 400,
    nameSize: 96,
    stepSize: 44,
  },
  post: {
    width: 1080,
    height: 1440,
    top: 64,
    bottom: 64,
    side: 64,
    gap: 40,
    qrSize: 480,
    qrSmallSize: 300,
    nameSize: 84,
    stepSize: 40,
  },
};

export function kitLayout(format: KitFormat): KitLayout {
  return LAYOUTS[format];
}

/** The picture names a path can carry, for example "reservar-historia" or "como-reservar-post". */
const KIT_IMAGES = new Map<string, KitImage>(
  KIT_PIECES.flatMap((piece) =>
    KIT_FORMATS.map((format): [string, KitImage] => [`${piece}-${format}`, { piece, format }]),
  ),
);

/** The picture a name names, or null for a piece or format the kit does not draw. */
export function parseKitImage(name: string): KitImage | null {
  const image = KIT_IMAGES.get(name);
  return image ? { ...image } : null;
}

/** The name a downloaded picture gets: vibe-<slug>-<piece>-<format>.png. */
export function kitFileName(slug: string, image: KitImage): string {
  return `vibe-${slug}-${image.piece}-${image.format}.png`;
}

/** Only ?descargar=1 asks for the picture as a download. */
export function wantsDownload(query: URLSearchParams): boolean {
  return query.get('descargar') === '1';
}
