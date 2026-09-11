/**
 * Build-time only: this runs in Astro's frontmatter, never in the browser.
 *
 * The dates in the structured data used to be typed by hand, and they drifted
 * five months behind the site. Freshness is one of the few signals a generative
 * engine can check cheaply, so a stale `dateModified` is worse than none.
 *
 * The last commit date is the honest answer — it moves when the content moves,
 * not every time someone redeploys an unchanged site.
 */
import { execSync } from 'node:child_process';

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

function lastCommitDate(): string | null {
  try {
    const out = execSync('git log -1 --format=%cs', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    return ISO_DATE.test(out) ? out : null;
  } catch {
    /* No git history here — a shallow or archive-based build. */
    return null;
  }
}

export const PUBLISHED = '2026-01-01';

export const LAST_MODIFIED = lastCommitDate() ?? new Date().toISOString().slice(0, 10);
