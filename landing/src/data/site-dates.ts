/**
 * Build-time only: the dates the structured data and the sitemap state.
 *
 * Three different dates, on purpose:
 *
 * - pageDateModified(route): each page's own date, from src/data/page-dates.json. That file
 *   holds the day the page's rendered output last changed (scripts/build-markdown.mjs writes it).
 *   This is what a page's dateModified and its sitemap <lastmod> say.
 * - SITE_MODIFIED: the newest of those page dates, the date of the latest content change on the
 *   whole site. Only for the few things that describe the site as a whole (WebSite).
 * - LAST_MODIFIED: the date a change lands, which is the last commit's date (today when git is
 *   unavailable). The guide date mechanism stamps changed guides with it. It moves with every
 *   commit, so it is never a page's dateModified.
 *
 * PUBLISHED is the first commit that added the home page, the home's datePublished.
 */
import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

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

/** The date the home page first existed: `git log --diff-filter=A -- landing/src/pages/index.astro`. */
export const PUBLISHED = '2026-09-11';

export interface PageDate {
  hash: string;
  date: string;
}

export type PageDates = Record<string, PageDate>;

/**
 * The newest date in the table, or null when it is empty. ISO dates compare as strings.
 */
export function newestDate(dates: PageDates): string | null {
  let newest: string | null = null;
  for (const entry of Object.values(dates)) {
    if (ISO_DATE.test(entry.date) && (newest === null || entry.date > newest)) newest = entry.date;
  }
  return newest;
}

/** A page's own date, or undefined for a route the table does not know (server-rendered pages). */
export function pageDateOf(dates: PageDates, route: string): string | undefined {
  const entry = dates[route];
  return entry && ISO_DATE.test(entry.date) ? entry.date : undefined;
}

function readPageDates(): PageDates {
  try {
    return JSON.parse(readFileSync(join(process.cwd(), 'src', 'data', 'page-dates.json'), 'utf8')) as PageDates;
  } catch {
    return {};
  }
}

const PAGE_DATES = readPageDates();

export const LAST_MODIFIED = lastCommitDate() ?? new Date().toISOString().slice(0, 10);

export const SITE_MODIFIED = newestDate(PAGE_DATES) ?? LAST_MODIFIED;

/** A page's dateModified: its own date when the table has it, else the site's newest date. */
export function pageDateModified(route: string): string {
  return pageDateOf(PAGE_DATES, route) ?? SITE_MODIFIED;
}
