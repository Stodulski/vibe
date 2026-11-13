import { TableFrameSkeleton } from '@/shared/components/common/TableFrameSkeleton';
import { PRODUCT_TABLE_COLUMNS } from './productLayout';

/** Loading shape of `ProductTable`. */
export function ProductTableSkeleton() {
  return <TableFrameSkeleton columns={PRODUCT_TABLE_COLUMNS} rows={5} />;
}
