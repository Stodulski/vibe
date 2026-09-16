import { useState } from 'react';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { CourtCard } from './CourtCard';
import { CourtTable } from './CourtTable';
import { CourtForm } from './CourtForm';
import { PriceConfig } from './PriceConfig';
import type { CourtWithPrices } from '@/shared/types/api.types';

interface CourtGridProps {
  courts: CourtWithPrices[];
  complexId: string;
}

export function CourtGrid({ courts, complexId }: CourtGridProps) {
  // The id, not the court object: a realtime refetch, another tab's edit, or
  // `useUpdateCourt`'s own optimistic update replaces `courts` with a new
  // array while a dialog is open, and a court captured at click time would
  // never see any of that — `PriceConfig` would keep computing its rows from
  // the price list that was current the moment "Precios" was clicked.
  // Looking the court up by id from the live `courts` prop on every render
  // means the open dialog always reflects the latest data.
  const [editId, setEditId] = useState<string | null>(null);
  const [priceId, setPriceId] = useState<string | null>(null);

  const editCourt = courts.find((c) => c.id === editId);
  const priceCourt = courts.find((c) => c.id === priceId);

  // A grid below `xl`, a dense table from `xl` up — not "style this
  // differently at that width" (a class) but "render a different
  // component" (see useMediaQuery's own doc comment), so the switch reads
  // the breakpoint the same way. `useSyncExternalStore` under the hood
  // means this is already correct on the first paint — nothing flashes the
  // wrong layout — and only one tree is ever mounted: each row otherwise
  // owns its own `useUpdateCourt`/`useDeleteCourt`, and a court's toggle or
  // delete dialog would exist twice if both the card and the table row for
  // it mounted at once.
  const isTableLayout = useMediaQuery('(min-width: 1280px)');

  return (
    <>
      {isTableLayout ? (
        <CourtTable
          courts={courts}
          complexId={complexId}
          onEdit={(c) => {
            setEditId(c.id);
          }}
          onPrices={(c) => {
            setPriceId(c.id);
          }}
        />
      ) : (
        // A grid, not a column. Twelve courts stacked in a 768px ribbon left
        // most of a desktop empty and turned a glanceable list into a scroll.
        // Two columns from `lg`, held back from `sm` because a card narrower
        // than about 400px starts truncating the price ranges — which are
        // the reason the card exists.
        <div className="grid w-full grid-cols-1 gap-3 lg:grid-cols-2">
          {courts.map((court) => (
            <CourtCard
              key={court.id}
              court={court}
              complexId={complexId}
              onEdit={(c) => {
                setEditId(c.id);
              }}
              onPrices={(c) => {
                setPriceId(c.id);
              }}
            />
          ))}
        </div>
      )}

      <CourtForm
        open={!!editCourt}
        onClose={() => {
          setEditId(null);
        }}
        complexId={complexId}
        court={editCourt}
      />

      {priceCourt && (
        <PriceConfig
          key={priceCourt.id}
          open={!!priceCourt}
          onClose={() => {
            setPriceId(null);
          }}
          complexId={complexId}
          court={priceCourt}
        />
      )}
    </>
  );
}
