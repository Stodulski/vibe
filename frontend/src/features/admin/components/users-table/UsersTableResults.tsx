import type { RefObject } from 'react';
import { Loader2 } from 'lucide-react';
import type { AdminUserRow } from '@/shared/types/api.types';
import { ResultCountAnnouncer } from '@/shared/components/common/ResultCountAnnouncer';
import { MobileUserCard } from './MobileUserCard';
import { DesktopUsersTable } from './DesktopUsersTable';

interface UsersTableResultsProps {
  users: AdminUserRow[];
  hasNextPage: boolean | undefined;
  isFetchingNextPage: boolean;
  sentinelRef: RefObject<HTMLDivElement | null>;
  onRowClick: (id: string) => void;
}

export function UsersTableResults({
  users,
  hasNextPage,
  isFetchingNextPage,
  sentinelRef,
  onRowClick,
}: UsersTableResultsProps) {
  return (
    <>
      <ResultCountAnnouncer count={users.length} />

      {/* Mobile card view */}
      <div className="flex flex-col gap-2 md:hidden">
        {users.map((user) => (
          <MobileUserCard
            key={user.id}
            user={user}
            onClick={() => {
              onRowClick(user.id);
            }}
          />
        ))}
      </div>

      <DesktopUsersTable users={users} />

      {hasNextPage && (
        <div ref={sentinelRef} className="flex justify-center py-4">
          {isFetchingNextPage && <Loader2 className="size-5 animate-spin text-text-tertiary" />}
        </div>
      )}
    </>
  );
}
