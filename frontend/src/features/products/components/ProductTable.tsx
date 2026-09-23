import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/shared/components/ui/table';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ProductTableRow } from './product-table/ProductTableRow';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

const HEAD_CELL = 'text-micro text-text-tertiary h-9 font-medium tracking-wider whitespace-nowrap uppercase';

interface ProductTableProps {
  products: Product[];
  onSelect: (product: Product) => void;
  onEdit: (product: Product) => void;
  onRestock: (product: Product) => void;
  onAdjust: (product: Product) => void;
  onToggleActive: (product: Product) => void;
}

/** The catalog as a dense table — the `lg`-and-up reading of `ProductGrid`'s cards. Same shape as `ClientTable`. */
export function ProductTable({ products, onSelect, onEdit, onRestock, onAdjust, onToggleActive }: ProductTableProps) {
  return (
    <div className="border-border-default bg-bg-subtle w-full overflow-hidden rounded-2xl border [&_td]:px-4 [&_td:first-child]:pl-6 [&_td:last-child]:pr-6 [&_th]:px-4 [&_th:first-child]:pl-6 [&_th:last-child]:pr-6">
      <Table>
        <TableHeader className="bg-bg-elevated">
          <TableRow className="border-border-subtle hover:bg-transparent">
            <TableHead className={HEAD_CELL}>{t.products.nameField}</TableHead>
            <TableHead className={HEAD_CELL}>{t.products.categoryLabel}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.products.priceLabel}</TableHead>
            <TableHead className={`${HEAD_CELL} text-right`}>{t.products.stockLabel}</TableHead>
            <TableHead className={HEAD_CELL} />
            <TableHead className={`${HEAD_CELL} text-right`}>{t.common.actions}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="whitespace-nowrap">
          {products.map((product) => (
            <ProductTableRow
              key={product.id}
              product={product}
              onSelect={onSelect}
              onEdit={onEdit}
              onRestock={onRestock}
              onAdjust={onAdjust}
              onToggleActive={onToggleActive}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
