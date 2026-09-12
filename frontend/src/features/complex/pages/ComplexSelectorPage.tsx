import { AlertTriangle, LogOut } from 'lucide-react';
import { useLogout } from '@/features/auth';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { SkeletonComplexSelector } from '@/shared/components/common/Skeletons';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { AppHeader } from '@/shared/components/layout/AppHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex } from '@/shared/types/api.types';
import { ComplexGrid } from './complex-selector/ComplexGrid';
import { NoComplexesState } from './complex-selector/NoComplexesState';
import { useComplexSelector } from './complex-selector/useComplexSelector';
import { MeshBackdrop } from '@/shared/components/layout/MeshBackdrop';

const t = ES_AR;

// Shared shell for the loading and error states: same backdrop, header and
// centered content column as the loaded page, just with a different body.
function ComplexSelectorShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="bg-bg-base relative flex min-h-dvh flex-col overflow-x-hidden">
      <MeshBackdrop />
      <AppHeader />
      <div className="flex flex-1 flex-col items-center px-4 pt-6 pb-12 sm:pt-16 sm:pb-20">{children}</div>
    </div>
  );
}

interface ComplexSelectorLoadedProps {
  complexes: Complex[];
  onSelectComplex: (complex: Complex) => void;
  onAddComplex: () => void;
  onLogout: () => void;
  isLoggingOut: boolean;
}

function ComplexSelectorLoaded({
  complexes,
  onSelectComplex,
  onAddComplex,
  onLogout,
  isLoggingOut,
}: ComplexSelectorLoadedProps) {
  const hasComplexes = complexes.length > 0;

  return (
    <div className="bg-bg-base relative flex min-h-dvh flex-col overflow-x-hidden">
      <MeshBackdrop />
      <AppHeader>
        <button
          onClick={onLogout}
          disabled={isLoggingOut}
          className="text-text-tertiary hover:bg-bg-elevated hover:text-text-primary flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors"
        >
          <LogOut className="size-4" aria-hidden="true" />
          <span className="hidden sm:inline">{t.auth.logout}</span>
          <span className="sr-only sm:hidden">{t.auth.logout}</span>
        </button>
      </AppHeader>

      {/* Content */}
      <div className="flex flex-1 flex-col items-center px-4 pt-6 pb-12 sm:pt-16 sm:pb-20">
        <div className="w-full max-w-2xl">
          {/* Title only. With complexes, a grid of clickable cards under "Tus
              complejos" does not need to be told it can be tapped. Without
              them, the line said "Creá tu primer complejo para comenzar"
              directly above NoComplexesState, which says the same thing and
              carries the button. */}
          <div className="mb-6 text-center sm:mb-8">
            <h1 className="font-display text-text-primary text-xl font-bold sm:text-3xl">
              {t.complex.selectComplexTitle}
            </h1>
          </div>

          {hasComplexes ? (
            <ComplexGrid complexes={complexes} onSelect={onSelectComplex} onAddComplex={onAddComplex} />
          ) : (
            <NoComplexesState onAddComplex={onAddComplex} />
          )}
        </div>
      </div>
    </div>
  );
}

export default function ComplexSelectorPage() {
  usePageTitle(t.complex.selectComplexTitle);
  const { complexes, isLoading, isError, refetch, handleSelectComplex, handleAddComplex } = useComplexSelector();
  const logoutMutation = useLogout();

  if (isLoading) {
    return (
      <ComplexSelectorShell>
        <SkeletonComplexSelector />
      </ComplexSelectorShell>
    );
  }

  if (isError) {
    return (
      <ComplexSelectorShell>
        <EmptyState
          icon={AlertTriangle}
          title={t.common.loadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void refetch();
          }}
        />
      </ComplexSelectorShell>
    );
  }

  return (
    <ComplexSelectorLoaded
      complexes={complexes ?? []}
      onSelectComplex={handleSelectComplex}
      onAddComplex={handleAddComplex}
      onLogout={() => {
        logoutMutation.mutate();
      }}
      isLoggingOut={logoutMutation.isPending}
    />
  );
}
