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
                  className="text-text-primary focus-visible:ring-primary-500/50 font-medium hover:underline focus-visible:rounded-sm focus-visible:ring-2 focus-visible:outline-none"
                >
                  {complex.name}
                </Link>
              </TableCell>
              <TableCell className="text-text-secondary text-sm">{complex.city}</TableCell>
              <TableCell>
                <div>
                  <p className="text-text-secondary text-sm">{complex.owner_name}</p>
                  <p className="text-text-tertiary text-xs">{complex.owner_email}</p>
                </div>
              </TableCell>
              <TableCell className="text-text-secondary text-center text-sm">{complex.courts_count}</TableCell>
              <TableCell className="text-center">
                {complex.mp_connected ? (
                  <CheckCircle className="text-success-text mx-auto size-4" aria-label={t.mp.connected} />
                ) : (
                  <XCircle className="text-text-tertiary mx-auto size-4" aria-label={t.mp.notConnected} />
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
