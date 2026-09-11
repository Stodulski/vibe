import { Search, AlertCircle } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { ES_AR } from '@/shared/i18n/es_AR';
import { UsersTableFilters } from './users-table/UsersTableFilters';
import { UsersTableSkeleton } from './users-table/UsersTableSkeleton';
import { UsersTableResults } from './users-table/UsersTableResults';
import { useUsersTableState } from './users-table/useUsersTableState';

const t = ES_AR;

export function UsersTable() {
  const {
    searchInput,
    setSearchInput,
    roleFilter,
    setRoleFilter,
    users,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    isFetchingNextPage,
    sentinelRef,
    handleRowClick,
  } = useUsersTableState();

  return (
    <div className="space-y-4">
      <UsersTableFilters
        searchInput={searchInput}
        onSearchChange={setSearchInput}
        roleFilter={roleFilter}
        onRoleFilterChange={setRoleFilter}
      />

      {isError ? (
        <EmptyState
          icon={AlertCircle}
          title={t.common.error}
          description={t.admin.users.loadError}
          actionLabel={t.common.refresh}
          onAction={() => {
            void refetch();
          }}
        />
      ) : isLoading ? (
        <UsersTableSkeleton />
      ) : users.length === 0 ? (
        <EmptyState icon={Search} title={t.admin.users.noUsers} description={t.common.tryAnotherSearch} />
      ) : (
        <UsersTableResults
          users={users}
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          sentinelRef={sentinelRef}
          onRowClick={handleRowClick}
        />
      )}
    </div>
  );
}
