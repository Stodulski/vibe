/**
 * The kit picture endpoint's answer. A piece or format the kit does not draw is a 404 before
 * anything is read. Any other picture is the share image's own answer, with its cache and its
 * failures, plus a download name when ?descargar=1 asks for one, and a smaller picture when ?email=1 asks for
 * the email size. The API read and the render are
 * passed in, so node:test can drive the answers with stubs.
 */
import type { ShareFacts } from '../complex-share.ts';
import { complexImageResponse, failure } from '../og/complex-image.ts';
import { emailWidth, kitFileName, parseKitImage, wantsDownload, type KitImage } from './kit-image.ts';

export interface KitImageDeps {
  /** Reads the complex and returns its card facts. Throws ApiError as the public API client does. */
  readFacts(slug: string): Promise<ShareFacts>;
  /** Draws the picture; width, when given, is the width it is rasterized at instead of its own. */
  render(facts: ShareFacts, slug: string, image: KitImage, width?: number): Promise<ArrayBuffer>;
}

export async function kitImageResponse(
  slug: string,
  imageName: string,
  query: URLSearchParams,
  deps: KitImageDeps,
): Promise<Response> {
  const image = parseKitImage(imageName);
  if (!image) return failure(404, 'Imagen no encontrada');
  const width = emailWidth(query);
  const headers = wantsDownload(query)
    ? { 'Content-Disposition': `attachment; filename="${kitFileName(slug, image)}"` }
    : {};
  return complexImageResponse(
    slug,
    {
      readFacts: (complexSlug) => deps.readFacts(complexSlug),
      render: (facts) => deps.render(facts, slug, image, width),
    },
    { headers },
  );
}
