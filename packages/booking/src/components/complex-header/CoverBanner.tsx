import { CoverPlaceholder } from './CoverPlaceholder';

interface CoverBannerProps {
  coverUrl: string | null;
  /** Describes the photo for search engines and screen readers; see `complexCoverAlt`. */
  alt: string;
}

/**
 * The club's cover, always present and always 16:9, filled edge to edge.
 *
 * The upload flow crops every cover to exactly 16:9 (the owner picks the
 * framing), so the photo fills the box. Covers uploaded before the cropper
 * existed may have another ratio; those are centre-cropped by `object-cover`
 * until the owner uploads again. The placeholder keeps the same box on
 * purpose. A club without a photo yet must not look like a different page
 * than a club with one, and the box must not jump in height the day the
 * owner uploads a picture.
 *
 * The cost is known: on a 1023×768 laptop the stacked placeholder is 608×342,
 * 45% of the screen's height, and the first bookable hour lands nearly two
 * screens down. The product decision was to keep the ratio anyway.
 *
 * Where the box sits is the caller's decision and changes with the viewport:
 * a banner above the name on a phone, a column beside it on a desktop.
 */
export function CoverBanner({ coverUrl, alt }: CoverBannerProps) {
  return (
    <div className="bg-bg-subtle relative aspect-[16/9] w-full overflow-hidden rounded-xl">
      {coverUrl ? (
        // width/height are the size the upload pipeline crops covers to; the
        // 16:9 box above already prevents the shift, this just makes the ratio
        // explicit to the browser (PERF-08). The alt is descriptive rather than
        // empty: the photo is content, and Google Images indexes a complex's
        // cover through its <img> and alt text.
        <img
          src={coverUrl}
          alt={alt}
          loading="eager"
          fetchPriority="high"
          decoding="async"
          width={1280}
          height={720}
          className="h-full w-full object-cover"
        />
      ) : (
        <CoverPlaceholder />
      )}
      {/* A short fade at the foot, only where the logo actually overlaps the
          photo — the stacked layout, below `lg`. Above that the cover is a
          column beside the logo and the fade would be dimming the club's
          photo for nothing.
          It began as `from-bg-subtle/90` through `via-bg-subtle/30`, darkening
          the whole lower half for the sake of text that used to sit on the
          cover and now lives below it. */}
      <div className="from-bg-subtle/80 absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t to-transparent lg:hidden" />
    </div>
  );
}
