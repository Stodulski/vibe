/**
 * Content negotiation for agents. The same URL answers with HTML for browsers and with the
 * page's Markdown twin for clients that ask for it.
 *
 * The helpers live in landing/middleware.ts, because the Vercel Edge bundle cannot import a
 * local .ts module; this file re-exports them for the Astro middleware and the tests.
 */
import { config, markdownTwinPath, notFoundTarget, NOT_FOUND_TARGET, wantsMarkdown } from '../../../middleware.ts';

export { markdownTwinPath, notFoundTarget, NOT_FOUND_TARGET, wantsMarkdown };

/**
 * The routes the Vercel middleware runs on. A `:param` matches one non-empty segment.
 * A new public page needs its line there, or it keeps answering HTML to agents.
 */
export const MARKDOWN_MATCHER: readonly string[] = config.matcher;

/** Whether the Vercel matcher runs on this route (a path, not a twin). */
export function matcherCovers(route: string): boolean {
  const path = route.replace(/\/+$/, '') || '/';
  const segments = path.split('/');
  return MARKDOWN_MATCHER.some((pattern) => {
    const want = pattern.split('/');
    if (want.length !== segments.length) return false;
    return want.every((part, i) => (part.startsWith(':') ? segments[i] !== '' : part === segments[i]));
  });
}
