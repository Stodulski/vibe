import { Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ProductStatusFilter } from './useProductsPage';

const t = ES_AR;

interface ProductsSearchBarProps {
  search: string;
  onSearchChange: (value: string) => void;
  filter: ProductStatusFilter;
  onFilterChange: (filter: ProductStatusFilter) => void;
}

export function ProductsSearchBar({ search, onSearchChange, filter, onFilterChange }: ProductsSearchBarProps) {
  return (
    <div className="flex flex-col gap-2.5 sm:flex-row sm:items-center">
      <IconInput
        icon={Search}
        value={search}
        onChange={(e) => {
          onSearchChange(e.target.value);
        }}
        placeholder={t.products.searchPlaceholder}
        aria-label={t.products.searchPlaceholder}
      />
      <div className="flex shrink-0 gap-1.5" role="group" aria-label={t.common.filters}>
        <Button
          type="button"
          size="sm"
          variant={filter === 'active' ? 'default' : 'outline'}
          aria-pressed={filter === 'active'}
          onClick={() => {
            onFilterChange('active');
          }}
        >
          {t.products.filterActive}
        </Button>
        <Button
          type="button"
          size="sm"
          variant={filter === 'inactive' ? 'default' : 'outline'}
          aria-pressed={filter === 'inactive'}
          onClick={() => {
            onFilterChange('inactive');
          }}
        >
          {t.products.filterInactive}
        </Button>
      </div>
    </div>
  );
}
