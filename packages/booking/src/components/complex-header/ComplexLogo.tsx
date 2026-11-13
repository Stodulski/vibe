import { complexLogoAlt } from '../../lib/complexImageAlt';

interface ComplexLogoProps {
  logoUrl: string | null;
  name: string;
}

/**
 * The club's logo, climbing over the bottom edge of the cover.
 *
 * At every width: cover, logo overlapping it, then the name. That grouping
 * stays together as one block and simply becomes the left-hand column on a
 * wide screen, rather than rearranging itself into a different relationship
 * at each breakpoint.
 */
export function ComplexLogo({ logoUrl, name }: ComplexLogoProps) {
  return (
    // Indented from the cover's left edge at every width. Flush against it,
    // the logo reads as falling off the photo rather than resting on it.
    <div className="-mt-10 mb-4 ml-4 sm:-mt-12">
      {logoUrl ? (
        // The upload flow crops logos to a square, so the image fills the tile.
        // Eager on purpose: this is above the fold on the storefront. The
        // intrinsic size is the square the upload pipeline crops logos to, so
        // the box is reserved before the file arrives (PERF-08).
        <div className="border-bg-subtle bg-bg-elevated relative size-20 overflow-hidden rounded-2xl border-2 shadow-md sm:size-24">
          <img
            src={logoUrl}
            alt={complexLogoAlt(name)}
            loading="eager"
            decoding="async"
            width={512}
            height={512}
            className="h-full w-full object-cover"
          />
        </div>
      ) : (
        // Opaque on purpose: a translucent tint let the cover show through the
        // part of the tile that overlaps it. This is the same 10% green, mixed
        // into the surface colour instead of into transparency.
        <div className="border-bg-subtle text-primary-500 flex size-20 items-center justify-center rounded-2xl border-2 bg-[color-mix(in_oklab,var(--color-primary-500)_10%,var(--color-bg-subtle))] text-2xl font-bold shadow-md sm:size-24 sm:text-3xl">
          {name.charAt(0).toUpperCase()}
        </div>
      )}
    </div>
  );
}
