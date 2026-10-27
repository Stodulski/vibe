/**
 * The Instagram glyph, as a drop-in replacement for a lucide icon.
 *
 * lucide has no brand icons (see `WhatsappIcon`). This is the camera outline,
 * drawn with lucide's own 24px grid and 2px stroke so it sits beside the other
 * contact icons at the same weight. Stroke is `currentColor`, so callers colour
 * it with a text class like any lucide icon.
 */
export function InstagramIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="2" y="2" width="20" height="20" rx="5" />
      <circle cx="12" cy="12" r="4" />
      <line x1="17.5" y1="6.5" x2="17.51" y2="6.5" />
    </svg>
  );
}
