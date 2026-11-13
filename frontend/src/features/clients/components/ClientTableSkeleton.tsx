import { TableFrameSkeleton } from '@/shared/components/common/TableFrameSkeleton';
import { CLIENT_TABLE_COLUMNS } from './clientLayout';

/** Loading shape of `ClientTable`. */
export function ClientTableSkeleton() {
  return <TableFrameSkeleton columns={CLIENT_TABLE_COLUMNS} />;
}
