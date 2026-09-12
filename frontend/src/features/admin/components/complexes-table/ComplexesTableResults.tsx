import type { RefObject } from 'react';
import { Loader2 } from 'lucide-react';
import type { AdminComplexRow } from '@/shared/types/api.types';
import { MobileComplexCard } from './MobileComplexCard';
import { DesktopComplexesTable } from './DesktopComplexesTable';

interface ComplexesTableResultsProps {
  complexes: AdminComplexRow[];
  hasNextPage: boolean | undefined;
  isFetchingNextPage: boolean;
  sentinelRef: RefObject<HTMLDivElement | null>;
  onRowClick: (id: string) => void;
}

export function ComplexesTableResults({
  complexes,
  hasNextPage,
  isFetchingNextPage,
  sentinelRef,
  onRowClick,
}: ComplexesTableResultsProps) {
  return (
    <>
      {/* Mobile card view */}
      <div className="flex flex-col gap-2 md:hidden">
        {complexes.map((complex) => (
          <MobileComplexCard
            key={complex.id}
            complex={complex}
            onClick={() => {
              onRowClick(complex.id);
            }}
          />
        ))}
      </div>

      <DesktopComplexesTable complexes={complexes} />

      {hasNextPage && (
        <div ref={sentinelRef} className="flex justify-center py-4">
          {isFetchingNextPage && <Loader2 className="size-5 animate-spin text-text-tertiary" />}
        </div>
      )}
    </>
  );
}
