/**
 * Draws one kit picture as a PNG: the words, the QR code and the layout of a piece in a format.
 * The fonts, the wasm, the backdrop photos and the Vibe logo (data URLs) arrive from the caller, as the fonts do
 * for the share card.
 */
import type { ShareFacts } from '../complex-share.ts';
import { rasterize, type CardAssets } from '../og/render.ts';
import { kitBackdrop, type KitBackdrop } from './backdrop.ts';
import { kitLayout, type KitImage } from './kit-image.ts';
import { kitText } from './kit-text.ts';
import { kitTree } from './kit-tree.ts';
import { qrSvgDataUrl } from './qr.ts';

export function renderKitImage(
  facts: ShareFacts,
  slug: string,
  image: KitImage,
  assets: CardAssets,
  photos: Readonly<Record<KitBackdrop, string>>,
  logo: string,
): Promise<ArrayBuffer> {
  const text = kitText(facts, slug, image.piece);
  const layout = kitLayout(image.format);
  const tree = kitTree(text, qrSvgDataUrl(text.url), image, layout, { photo: photos[kitBackdrop(facts.sports)], logo });
  return rasterize(tree, layout.width, layout.height, assets);
}
