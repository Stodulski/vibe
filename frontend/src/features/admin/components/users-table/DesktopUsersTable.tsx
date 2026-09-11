import { Link } from 'react-router-dom';
import { Building2 } from 'lucide-react';
import { Badge } from '@/shared/components/ui/badge';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { DesktopTableShell } from '@/shared/components/common/DesktopTableShell';
import { Table, TableBody, TableCell, TableRow } from '@/shared/components/ui/table';
import { formatDateCompact } from '@/shared/lib/utils';
import type { AdminUserRow } from '@/shared/types/api.types';
import { UsersTableHead } from './UsersTableHead';
import { getRoleLabel } from './userTableUtils';

interface DesktopUsersTableProps {
  users: AdminUserRow[];
}

export function DesktopUsersTable({ users }: DesktopUsersTableProps) {
  return (
    <DesktopTableShell>
      <Table>
        <UsersTableHead />
        <TableBody>
          {users.map((user) => (
            <TableRow key={user.id} className="hover:bg-bg-elevated/50 transition-colors">
              <TableCell>
                <Link
                  to={`/admin/users/${user.id}`}
                  className="font-medium text-text-primary hover:underline focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
                >
                  {user.first_name} {user.last_name}
                </Link>
              </TableCell>
              <TableCell className="text-text-secondary text-sm">{user.email}</TableCell>
              <TableCell>
                <Badge variant="secondary" className="text-xs">
                  {getRoleLabel(user.role)}
                </Badge>
              </TableCell>
              <TableCell className="text-center">
                <div className="flex items-center justify-center gap-1 text-text-secondary">
                  <Building2 className="size-3.5" aria-hidden="true" />
                  <span className="text-sm">{user.complex_count}</span>
                </div>
              </TableCell>
              <TableCell className="text-center">
                <ActiveStatusBadge isActive={user.is_active} className="text-xs" />
              </TableCell>
              <TableCell className="text-text-tertiary text-sm">{formatDateCompact(user.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </DesktopTableShell>
  );
}
