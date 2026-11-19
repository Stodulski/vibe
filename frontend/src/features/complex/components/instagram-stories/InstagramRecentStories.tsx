import { ES_AR } from '@/shared/i18n/es_AR';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { cn } from '@/shared/lib/utils';
import type { InstagramStory } from '../../schemas/instagramStories.schema';

const t = ES_AR.instagram.stories;

const STATUS_TONE: Record<InstagramStory['status'], string> = {
  published: 'text-success-text',
  pending: 'text-text-secondary',
  skipped: 'text-text-tertiary',
  failed: 'text-error-text',
};

/**
 * Why a story did not go out, in the owner's words. A skipped story maps the
 * server's stable reason code (an unknown one shows no explanation); a failed
 * one gets a generic cause, never the technical reason the server keeps.
 */
function storyExplanation(story: InstagramStory): string | null {
  if (story.status === 'failed') return t.failedReason;
  if (story.status === 'skipped' && story.reason && Object.hasOwn(t.skipReasons, story.reason)) {
    return t.skipReasons[story.reason as keyof typeof t.skipReasons];
  }
  return null;
}

/** The last stories, newest first, as the server lists them. */
export function InstagramRecentStories({ stories }: { stories: readonly InstagramStory[] }) {
  return (
    <section aria-labelledby="ig-recent-title" className="space-y-2">
      <h3 id="ig-recent-title" className="text-text-primary text-sm font-semibold">
        {t.recentTitle}
      </h3>
      {stories.length === 0 ? (
        <p className="text-text-tertiary text-xs">{t.recentEmpty}</p>
      ) : (
        <ul className="divide-border-subtle divide-y">
          {stories.map((story, i) => {
            const explanation = storyExplanation(story);
            return (
              // A story has no id of its own in the contract; the list is
              // replaced whole on every read, so the position is stable enough.
              <li key={i} className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5 py-2 text-xs">
                <span className="text-text-primary min-w-0">{t.kinds[story.kind]}</span>
                <span className="flex items-baseline gap-2">
                  <span className={cn('font-medium', STATUS_TONE[story.status])}>{t.statuses[story.status]}</span>
                  <span className="text-text-tertiary tabular-nums">
                    {formatVenueDayTime(story.published_at ?? story.scheduled_for)}
                  </span>
                </span>
                {explanation && <span className="text-text-tertiary basis-full">{explanation}</span>}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
