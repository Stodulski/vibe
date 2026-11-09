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
        // Same treatment as the cover: an opaque tile, the logo blurred and
        // dimmed behind, the logo whole on top. A transparent logo otherwise
        // shows the cover through its top half and the page through its
        // bottom half, and a non-square one loses its edges to the crop.
        <div className="border-bg-subtle bg-bg-elevated relative size-20 overflow-hidden rounded-2xl border-2 shadow-md sm:size-24">
          {/* Decorative: empty alt, hidden from assistive tech, same URL so no extra request. */}
          <img
            src={logoUrl}
            alt=""
            aria-hidden="true"
            decoding="async"
            className="absolute inset-0 h-full w-full scale-125 object-cover opacity-50 blur-md"
          />
          {/* Eager on purpose: this is above the fold on the storefront. The
              intrinsic size is the square the upload pipeline caps logos to,
              so the box is reserved before the file arrives (PERF-08). */}
          <img
            src={logoUrl}
            alt={complexLogoAlt(name)}
            loading="eager"
            decoding="async"
            width={512}
            height={512}
            className="relative h-full w-full object-contain p-1.5"
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
