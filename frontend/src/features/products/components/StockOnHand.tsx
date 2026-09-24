import { Infinity as InfinityIcon } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * A product's stock figure: the count when the product tracks stock, an
 * infinity sign when it does not. The sign replaces the "Sin control de
 * stock" text on screen (it did not fit a card at 320 px); the text stays for
 * screen readers and as the hover title.
 */
export function StockOnHand({ tracksStock, stockOnHand }: { tracksStock: boolean; stockOnHand: number }) {
  if (tracksStock) return <>{stockOnHand}</>;
  return (
    <span className="inline-flex items-center align-middle" title={t.products.noStockControl}>
      <InfinityIcon className="size-4" aria-hidden="true" />
      <span className="sr-only">{t.products.noStockControl}</span>
    </span>
  );
}
