import type { ReactNode } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.instagram.stories;

/** Heading and divider the stories section and its skeleton share, so the loaded one lands in place. */
export function InstagramStoriesShell({ children }: { children: ReactNode }) {
  return (
    <section aria-labelledby="ig-stories-title" className="border-border-subtle space-y-5 border-t pt-5">
      <div className="space-y-1">
        <h2 id="ig-stories-title" className="text-text-primary text-sm font-semibold">
          {t.title}
        </h2>
        <p className="text-text-secondary text-xs">{t.description}</p>
      </div>
      {children}
    </section>
  );
}
