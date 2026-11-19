import { ExternalLink } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { instagramDailyStoryUrl } from '@/shared/lib/publicPaths';
import { useInstagramStories } from '../../hooks/useInstagramStories';
import { InstagramRecentStories } from './InstagramRecentStories';
import { InstagramStoriesForm } from './InstagramStoriesForm';
import { InstagramStoriesSectionSkeleton } from './InstagramStoriesSectionSkeleton';
import { InstagramStoriesShell } from './InstagramStoriesShell';

const t = ES_AR;

function PreviewLink({ slug }: { slug: string }) {
  return (
    <Button asChild variant="outline" size="sm" className="w-full sm:w-auto">
      <a href={instagramDailyStoryUrl(slug)} target="_blank" rel="noopener noreferrer">
        <ExternalLink className="size-3.5" />
        {t.instagram.stories.preview}
      </a>
    </Button>
  );
}

/**
 * Automatic stories: which types Vibe publishes and when, plus the last ones.
 * Mounted only under a usable connection; every type starts off.
 */
export function InstagramStoriesSection({ complexId, slug }: { complexId: string; slug: string }) {
  const { data, isLoading, isError, refetch } = useInstagramStories(complexId);

  if (isLoading) return <InstagramStoriesSectionSkeleton />;
  if (isError || !data) {
    return (
      <InstagramStoriesShell>
        <div className="flex flex-col items-start gap-3">
          <p className="text-text-tertiary text-sm">{t.instagram.stories.loadError}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void refetch();
            }}
          >
            {t.common.refresh}
          </Button>
        </div>
      </InstagramStoriesShell>
    );
  }

  return (
    <InstagramStoriesShell>
      <InstagramStoriesForm complexId={complexId} settings={data.settings} preview={<PreviewLink slug={slug} />} />
      <InstagramRecentStories stories={data.recent} />
    </InstagramStoriesShell>
  );
}
