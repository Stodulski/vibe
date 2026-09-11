import { useState } from 'react';
import { AlertTriangle, Plus, Trophy } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { SkeletonCourtCard } from '@/shared/components/common/Skeletons';
import { CourtGrid, CourtForm, PriceConfig, useCourts } from '@/features/courts';
import { useSelectedComplex } from '@/features/complex';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

function CourtCardSkeletons() {
  return (
    <div className="grid w-full grid-cols-1 gap-3 lg:grid-cols-2 2xl:grid-cols-3">
      <SkeletonCourtCard />
      <SkeletonCourtCard />
      <SkeletonCourtCard />
    </div>
  );
}

interface CourtsPageListProps {
  complexId: string;
  courts: CourtWithPrices[] | undefined;
  isLoading: boolean;
  isError: boolean;
  refetch: () => void;
  onCreate: () => void;
}

function CourtsPageList({ complexId, courts, isLoading, isError, refetch, onCreate }: CourtsPageListProps) {
  if (isError) {
    return (
      <EmptyState
        icon={AlertTriangle}
        title={t.common.loadError}
        description={t.common.loadErrorDescription}
        actionLabel={t.layout.retry}
        onAction={refetch}
      />
    );
  }

  if (isLoading) return <CourtCardSkeletons />;

  if (!courts || courts.length === 0) {
    return (
      <EmptyState
        icon={Trophy}
        title={t.courts.noCourts}
        description={t.courts.noCourtsDescription}
        actionLabel={t.courts.create}
        onAction={onCreate}
      />
    );
  }

  return <CourtGrid courts={courts} complexId={complexId} />;
}

function CourtsPageContent({ complexId }: { complexId: string }) {
  const { data: courts, isLoading, isError, refetch } = useCourts(complexId);
  const [createOpen, setCreateOpen] = useState(false);
  // The id, not the court object — the same reasoning as `CourtGrid`'s own
  // `priceId`: looked up from the live `courts` query data on every render
  // instead of a snapshot that could go stale while the dialog is open.
  const [priceCourtId, setPriceCourtId] = useState<string | null>(null);
  const priceCourt = courts?.find((c) => c.id === priceCourtId) ?? null;

  return (
    <>
      <PageHeader
        title={t.courts.title}
        description={t.courts.pageDescription}
        action={{
          label: t.courts.create,
          onClick: () => {
            setCreateOpen(true);
          },
          icon: Plus,
        }}
      />

      <CourtsPageList
        complexId={complexId}
        courts={courts}
        isLoading={isLoading}
        isError={isError}
        refetch={() => {
          void refetch();
        }}
        onCreate={() => {
          setCreateOpen(true);
        }}
      />

      <CourtForm
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
        }}
        complexId={complexId}
        onCreated={(court) => {
          setPriceCourtId(court.id);
        }}
      />

      {priceCourt && (
        <PriceConfig
          open={!!priceCourt}
          onClose={() => {
            setPriceCourtId(null);
          }}
          complexId={complexId}
          court={priceCourt}
        />
      )}
    </>
  );
}

export default function CourtsPage() {
  usePageTitle(t.courts.title);
  const { selectedComplexId } = useSelectedComplex();

  // Guarded here so `CourtsPageContent` below — and the `useCourts`,
  // `CourtForm` and `PriceConfig` it owns — only ever mount once there's a
  // real complex id, instead of every one of them carrying its own `?? ''`
  // fallback for a "no complex yet" state that can't actually occur once
  // they're mounted.
  if (!selectedComplexId) {
    return (
      <div className="animate-fade-in">
        <PageHeader title={t.courts.title} description={t.courts.pageDescription} />
        <CourtCardSkeletons />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <CourtsPageContent complexId={selectedComplexId} />
    </div>
  );
}
