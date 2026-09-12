import type { RefObject } from 'react';
import { Loader2 } from 'lucide-react';
import type { AdminUserRow } from '@/shared/types/api.types';
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
          {isFetchingNextPage && <Loader2 className="text-text-tertiary size-5 animate-spin" />}
        </div>
      )}
    </>
  );
}
