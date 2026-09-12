import { Search, AlertCircle } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ResultCountAnnouncer } from '@/shared/components/common/ResultCountAnnouncer';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComplexesTableSkeleton } from './complexes-table/ComplexesTableSkeleton';
import { ComplexesTableResults } from './complexes-table/ComplexesTableResults';
import { useComplexesTableState } from './complexes-table/useComplexesTableState';

const t = ES_AR;

export function ComplexesTable() {
  const {
    searchInput,
    setSearchInput,
    complexes,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    isFetchingNextPage,
    sentinelRef,
    handleRowClick,
  } = useComplexesTableState();

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <IconInput
          icon={Search}
          placeholder={t.admin.complexes.searchPlaceholder}
          value={searchInput}
          onChange={(e) => {
            setSearchInput(e.target.value);
          }}
        />
      </div>

      {/* Beside the whole conditional, not inside the branch that draws the
          rows: the region has to exist before the count changes, and the
          empty branch is exactly the one a person filtering needs to hear
          (A11Y-07). */}
      <ResultCountAnnouncer count={isError || isLoading ? null : complexes.length} />

      {isError ? (
        <EmptyState
          icon={AlertCircle}
          title={t.common.error}
          description={t.admin.complexes.loadError}
          actionLabel={t.common.refresh}
          onAction={() => {
            void refetch();
          }}
        />
      ) : isLoading ? (
        <ComplexesTableSkeleton />
      ) : complexes.length === 0 ? (
        <EmptyState icon={Search} title={t.admin.complexes.noComplexes} description={t.common.tryAnotherSearch} />
      ) : (
        <ComplexesTableResults
          complexes={complexes}
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          sentinelRef={sentinelRef}
          onRowClick={handleRowClick}
        />
      )}
    </div>
  );
}
