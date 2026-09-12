import { Logo } from '@/shared/components/common/Logo';
import { MeshBackdrop } from './MeshBackdrop';
import type { ReactNode } from 'react';

interface AuthSplitLayoutProps {
  children: ReactNode;
}

/**
 * Shared shell for login/register: a full-bleed photo panel alongside the
 * form on desktop — the panel is a nice-to-have, not something worth
 * fighting for space on a 320px screen (Practical UI ch.2, design for the
 * smallest screen first), so mobile just gets the logo above the form.
 */
export function AuthSplitLayout({ children }: AuthSplitLayoutProps) {
  return (
    <div className="relative flex min-h-dvh flex-col bg-bg-base md:h-dvh md:overflow-hidden">
      <MeshBackdrop />

      {/* Desktop: a real 50/50 flex row — image and form grow equally, with
          a fixed minimum gap between them. The row's height is pinned to
          the viewport (md:h-dvh + overflow-hidden on the page), so the
          image never scrolls; if a form step has more fields than fit,
          only the form column scrolls internally (md:overflow-y-auto), not
          the whole page. */}
      <div className="relative z-10 flex flex-1 flex-col md:min-h-0 md:flex-row md:gap-8 md:py-8 md:pl-8">
        <div className="hidden md:flex md:grow md:basis-0">
          {/* 272 kB of decoration that a phone never shows: `lazy` lets the
              browser skip it while the column is display:none, and the
              intrinsic size (the file is 1600x2400) lets it reserve the box
              before the bytes arrive (PERF-08). */}
          <img
            src="/auth-hero.webp"
            alt=""
            loading="lazy"
            decoding="async"
            width={1600}
            height={2400}
            className="h-full w-full rounded-2xl object-cover"
          />
        </div>

        {/* Mobile: the logo is pinned to the top of the column and the card
            is centred in the space below it. It used to sit inside the card,
            one line above the content, so on the status pages (mail sent,
            verified, link expired) the brand mark and the page's own icon
            stacked into two emblems glued together. */}
        <div className="flex flex-1 flex-col px-5 pb-8 pt-6 sm:px-6 md:grow md:min-h-0 md:items-center md:justify-center md:overflow-y-auto md:py-8 md:pl-0 md:pr-8">
          <Logo size="lg" alt="" className="mx-auto md:hidden" />
          <div className="flex w-full flex-1 flex-col items-center justify-center md:flex-none">
            <div className="w-full max-w-md animate-auth-card-in md:max-w-sm lg:max-w-md">{children}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
