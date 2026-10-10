import { SearchX } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * Shown when `useComplexBySlug` 404s (the slug never resolved to a complex)
 * or the query settles with no data. The most common way a player lands
 * here is a stale WhatsApp link or bookmark to a complex that was renamed or
 * deleted, so when the URL still carries a slug this names it instead of a
 * generic "not found" — and gives a real way back, which the old plain-text
 * version never did.
 *
 * The "404" kicker and the message live inside one `<h2>` (via StatusHero's
 * `title`) rather than as two separate headings — one real heading, whose
 * accessible name still carries both, instead of a numeral pretending to be
 * the page's heading with the actual message demoted to a plain paragraph.
 */
export function NotFoundState({ slug }: { slug?: string }) {
  const description = slug
    ? `${t.publicBooking.complexNotFoundSlugPrefix} "${slug}"${t.publicBooking.complexNotFoundSlugSuffix}`
    : t.publicBooking.complexNotFoundDescription;

  return (
    <StatusHero
      icon={SearchX}
      tone="error"
      animated={false}
      title={
        <>
          <span className="text-text-tertiary block text-xs font-semibold tracking-widest uppercase">404</span>
          <span>{t.publicBooking.complexNotFound}</span>
        </>
      }
      description={description}
    >
      <Button asChild size="lg" className="mt-4 min-h-12 rounded-xl">
        <a href="/">{t.layout.backHome}</a>
      </Button>
    </StatusHero>
  );
}
