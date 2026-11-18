import { PageHeader } from '@/shared/components/common/PageHeader';
import { useSelectedComplex } from '@/features/complex';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_PIECES } from '@/shared/lib/publicPaths';
import { KitPieceSection } from '../components/KitPieceSection';

const t = ES_AR;

/** Owner page with the complex's social kit: the four images to download and post on their own networks. */
export default function SocialKitPage() {
  usePageTitle(t.navigation.socialKit);
  const { complex } = useSelectedComplex();

  // No slug until the selected complex has loaded, so there is nothing to link to yet (as on the dashboard).
  if (!complex?.slug) return null;

  return (
    <div className="space-y-8">
      <PageHeader title={t.navigation.socialKit} />
      <p className="text-text-secondary max-w-xl text-sm leading-relaxed">{t.navigation.socialKitIntro}</p>
      {SOCIAL_KIT_PIECES.map((piece) => (
        <KitPieceSection key={piece} piece={piece} slug={complex.slug} />
      ))}
    </div>
  );
}
