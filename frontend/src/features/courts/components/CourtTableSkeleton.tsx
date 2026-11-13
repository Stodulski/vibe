import { TableFrameSkeleton } from '@/shared/components/common/TableFrameSkeleton';
import { COURT_TABLE_COLUMNS } from './courtLayout';

/** Loading shape of `CourtTable`. */
export function CourtTableSkeleton() {
  return <TableFrameSkeleton columns={COURT_TABLE_COLUMNS} />;
}
