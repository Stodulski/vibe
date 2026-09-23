import { Pencil, PackagePlus, SlidersHorizontal, Ban, RotateCcw, MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Product } from '@/shared/types/api.types';

const t = ES_AR;

interface ProductActionsMenuProps {
  product: Product;
  onEdit: () => void;
  onRestock: () => void;
  onAdjust: () => void;
  onToggleActive: () => void;
}

/** Editar/Reponer/Ajustar/Desactivar-Reactivar behind one `⋯` trigger — same shape as `CourtRowActionsMenu`. Ajustar only for a stock-tracked product. */
export function ProductActionsMenu({ product, onEdit, onRestock, onAdjust, onToggleActive }: ProductActionsMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`${t.common.rowActionsLabel}: ${product.name}`}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onEdit}>
          <Pencil className="size-3.5" />
          {t.products.editAction}
        </DropdownMenuItem>
        {product.tracks_stock && (
          <DropdownMenuItem onClick={onRestock}>
            <PackagePlus className="size-3.5" />
            {t.products.restockAction}
          </DropdownMenuItem>
        )}
        {product.tracks_stock && (
          <DropdownMenuItem onClick={onAdjust}>
            <SlidersHorizontal className="size-3.5" />
            {t.products.adjustAction}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem {...(product.active ? { variant: 'destructive' as const } : {})} onClick={onToggleActive}>
          {product.active ? <Ban className="size-3.5" /> : <RotateCcw className="size-3.5" />}
          {product.active ? t.products.deactivateAction : t.products.reactivateAction}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
