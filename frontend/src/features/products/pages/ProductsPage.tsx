import { Plus } from 'lucide-react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { CashSectionTabs } from '@/shared/components/common/CashSectionTabs';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useProductsPage } from './products/useProductsPage';
import { ProductsSearchBar } from './products/ProductsSearchBar';
import { ProductsContent } from './products/ProductsContent';
import { PRODUCTS_BODY_CLASS } from './products/productsPageLayout';
import { ProductsPageDialogs } from './products/ProductsPageDialogs';

const t = ES_AR;

export default function ProductsPage() {
  usePageTitle(t.products.title);
  const state = useProductsPage();

  if (!state.complexId) return null;

  return (
    <div>
      <PageHeader
        title={t.products.title}
        action={{
          label: t.products.newProduct,
          icon: Plus,
          onClick: () => {
            state.setCreateOpen(true);
          },
        }}
      />
      <CashSectionTabs />

      <div className={PRODUCTS_BODY_CLASS}>
        <ProductsSearchBar
          search={state.search}
          onSearchChange={state.setSearch}
          filter={state.filter}
          onFilterChange={state.setFilter}
        />

        <ProductsContent
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          products={state.products}
          hasAnyProducts={state.hasAnyProducts}
          onCreate={() => {
            state.setCreateOpen(true);
          }}
          onSelect={state.goToDetail}
          onEdit={state.setEditingProduct}
          onRestock={state.setRestockTarget}
          onAdjust={state.setAdjustTarget}
          onToggleActive={state.setToggleTarget}
        />
      </div>

      <ProductsPageDialogs state={state} complexId={state.complexId} />
    </div>
  );
}
