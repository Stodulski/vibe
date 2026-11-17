import type { Sport } from '../../api/types';

/**
 * A sport drawn as its court seen from above: the shape a player knows the game by,
 * in thin lines that take the card's color.
 */
export function SportGlyph({ sport }: { sport: string }) {
  return (
    <svg viewBox="0 0 80 52" className="h-16 w-24" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="2" y="2" width="76" height="48" rx="3" />
      {LINES[sport as Sport] ?? <line x1="40" y1="2" x2="40" y2="50" />}
    </svg>
  );
}

const LINES: Partial<Record<Sport, React.ReactNode>> = {
  padel: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" strokeWidth="2.5" />
      <line x1="17" y1="2" x2="17" y2="50" />
      <line x1="63" y1="2" x2="63" y2="50" />
      <line x1="17" y1="26" x2="63" y2="26" />
    </>
  ),
  tennis: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" strokeWidth="2.5" />
      <line x1="2" y1="9" x2="78" y2="9" />
      <line x1="2" y1="43" x2="78" y2="43" />
      <line x1="20" y1="9" x2="20" y2="43" />
      <line x1="60" y1="9" x2="60" y2="43" />
      <line x1="20" y1="26" x2="60" y2="26" />
    </>
  ),
  pickleball: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" strokeWidth="2.5" />
      <line x1="30" y1="2" x2="30" y2="50" />
      <line x1="50" y1="2" x2="50" y2="50" />
      <line x1="2" y1="26" x2="30" y2="26" />
      <line x1="50" y1="26" x2="78" y2="26" />
    </>
  ),
  soccer: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" />
      <circle cx="40" cy="26" r="8" />
      <rect x="2" y="16" width="9" height="20" />
      <rect x="69" y="16" width="9" height="20" />
    </>
  ),
  hockey: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" />
      <circle cx="40" cy="26" r="6" />
      <path d="M2 12 A14 14 0 0 1 2 40" />
      <path d="M78 12 A14 14 0 0 0 78 40" />
    </>
  ),
  basketball: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" />
      <circle cx="40" cy="26" r="7" />
      <path d="M2 10 A20 20 0 0 1 2 42" />
      <path d="M78 10 A20 20 0 0 0 78 42" />
    </>
  ),
  volleyball: (
    <>
      <line x1="40" y1="2" x2="40" y2="50" strokeWidth="2.5" />
      <line x1="27" y1="2" x2="27" y2="50" />
      <line x1="53" y1="2" x2="53" y2="50" />
    </>
  ),
};
