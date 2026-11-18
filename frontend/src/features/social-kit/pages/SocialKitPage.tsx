import { PageHeader } from '@/shared/components/common/PageHeader';
import { useSelectedComplex } from '@/features/complex';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_PIECES } from '@/shared/lib/publicPaths';
import { KitPieceRow } from '../components/KitPieceRow';
import { KIT_LIST_CLASS } from '../components/kitLayout';

const t = ES_AR;

/** Owner page with the complex's social kit: one card per piece, each downloadable as a story or a post. */
export default function SocialKitPage() {
  usePageTitle(t.navigation.socialKit);
  const { complex } = useSelectedComplex();

  // No slug until the selected complex has loaded, so there is nothing to link to yet (as on the dashboard).
  if (!complex?.slug) return null;

  return (
    <div className="space-y-6">
      <PageHeader title={t.navigation.socialKit} />
      <p className="text-text-secondary max-w-xl text-sm leading-relaxed">{t.navigation.socialKitIntro}</p>
      <div className={KIT_LIST_CLASS}>
        {SOCIAL_KIT_PIECES.map((piece) => (
          <KitPieceRow key={piece} piece={piece} slug={complex.slug} />
        ))}
      </div>
    </div>
  );
}
