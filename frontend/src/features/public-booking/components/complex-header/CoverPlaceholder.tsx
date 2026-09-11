/**
 * What fills the cover's frame before a club uploads its own photo.
 *
 * Two things it deliberately is not. It is not the flat gradient that was
 * here before, which read as an image that had failed to load. And it is not
 * a stock photograph of a padel club — a real venue's courts shown on another
 * venue's page is a claim about premises the visitor has never seen, and they
 * have no way of knowing it isn't true.
 *
 * So it is drawn, not photographed: a court diagram in the brand's own colour,
 * on the brand's own dark. Anyone can tell at a glance that nobody took this
 * picture, which is exactly what a placeholder has to say. It also costs no
 * request and no kilobytes — it is markup — and it follows the theme instead
 * of fighting it.
 */
export function CoverPlaceholder() {
  return (
    <svg viewBox="0 0 160 90" preserveAspectRatio="xMidYMid slice" className="h-full w-full" aria-hidden="true">
      <rect width="160" height="90" style={{ fill: 'var(--color-bg-elevated)' }} />

      {/* Two soft washes, so the panel has some depth rather than reading as
          one flat fill. */}
      <circle cx="24" cy="14" r="46" style={{ fill: 'var(--color-primary-500)' }} opacity="0.10" />
      <circle cx="140" cy="82" r="38" style={{ fill: 'var(--color-primary-500)' }} opacity="0.07" />

      {/* A padel court from above: the outer box, the net, the service line
          and the two side lines. Enough to be recognised, too plain to be
          mistaken for a photograph of anywhere.
          It sits inside the middle third of the canvas on purpose. This
          drawing fills boxes of several different shapes — a full-width band
          on a phone, a shorter one beside the name on a desktop — and
          `slice` crops whatever does not fit. Drawn edge to edge, the court's
          own outline landed exactly on the crop line and read as a grid of
          loose lines. Kept small, it survives every crop whole. */}
      <g
        fill="none"
        style={{ stroke: 'var(--color-primary-500)' }}
        strokeWidth="0.5"
        strokeLinecap="round"
        opacity="0.45"
      >
        <rect x="55" y="31" width="50" height="28" rx="1" />
        <line x1="80" y1="31" x2="80" y2="59" strokeDasharray="1.5 2" />
        <line x1="55" y1="45" x2="105" y2="45" />
        <line x1="65" y1="31" x2="65" y2="59" />
        <line x1="95" y1="31" x2="95" y2="59" />
      </g>
    </svg>
  );
}
