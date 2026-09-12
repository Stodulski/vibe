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
        // Eager on purpose: this is above the fold on the storefront. The
        // intrinsic size is the square the upload pipeline caps logos to,
        // so the box is reserved before the file arrives (PERF-08).
        <img
          src={logoUrl}
          alt={name}
          loading="eager"
          decoding="async"
          width={512}
          height={512}
          className="border-bg-subtle size-20 rounded-2xl border-2 object-cover shadow-md sm:size-24"
        />
      ) : (
        <div className="border-bg-subtle bg-primary-500/10 text-primary-500 flex size-20 items-center justify-center rounded-2xl border-2 text-2xl font-bold shadow-md sm:size-24 sm:text-3xl">
          {name.charAt(0).toUpperCase()}
        </div>
      )}
    </div>
  );
}
