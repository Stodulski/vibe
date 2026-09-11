import { Link } from 'react-router-dom';
import { CheckCircle, XCircle } from 'lucide-react';
import { ActiveStatusBadge } from '@/shared/components/common/ActiveStatusBadge';
import { DesktopTableShell } from '@/shared/components/common/DesktopTableShell';
import { Table, TableBody, TableCell, TableRow } from '@/shared/components/ui/table';
import { formatDateCompact } from '@/shared/lib/utils';
import type { AdminComplexRow } from '@/shared/types/api.types';
import { ComplexesTableHead } from './ComplexesTableHead';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface DesktopComplexesTableProps {
  complexes: AdminComplexRow[];
}

export function DesktopComplexesTable({ complexes }: DesktopComplexesTableProps) {
  return (
    <DesktopTableShell>
      <Table>
        <ComplexesTableHead />
        <TableBody>
          {complexes.map((complex) => (
            <TableRow key={complex.id} className="hover:bg-bg-elevated/50 transition-colors">
              <TableCell>
                <Link
                  to={`/admin/complexes/${complex.id}`}
                  className="font-medium text-text-primary hover:underline focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
                >
                  {complex.name}
                </Link>
              </TableCell>
              <TableCell className="text-text-secondary text-sm">{complex.city}</TableCell>
              <TableCell>
                <div>
                  <p className="text-sm text-text-secondary">{complex.owner_name}</p>
                  <p className="text-xs text-text-tertiary">{complex.owner_email}</p>
                </div>
              </TableCell>
              <TableCell className="text-center text-sm text-text-secondary">{complex.courts_count}</TableCell>
              <TableCell className="text-center">
                {complex.mp_connected ? (
                  <CheckCircle className="size-4 text-green-400 mx-auto" aria-label={t.mp.connected} />
                ) : (
                  <XCircle className="size-4 text-text-tertiary mx-auto" aria-label={t.mp.notConnected} />
                )}
              </TableCell>
              <TableCell className="text-center">
                <ActiveStatusBadge isActive={complex.is_active} className="text-xs" />
              </TableCell>
              <TableCell className="text-text-tertiary text-sm">{formatDateCompact(complex.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </DesktopTableShell>
  );
}
