import { Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { Tag } from '@/shared/components/common/Tag';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ClientsSearchBarProps {
  searchInput: string;
  onSearchChange: (value: string) => void;
  clientCount: number;
  /** Holds the count pill's place while the list loads, so the input does not narrow when it appears. */
  isLoading?: boolean;
}

export function ClientsSearchBar({
  searchInput,
  onSearchChange,
  clientCount,
  isLoading = false,
}: ClientsSearchBarProps) {
  return (
    <div className="flex items-center gap-3">
      <IconInput
        icon={Search}
        value={searchInput}
        onChange={(e) => {
          onSearchChange(e.target.value);
        }}
        placeholder={t.common.search}
        aria-label={`${t.common.search} clientes`}
      />
      {isLoading && <Skeleton className="h-7 w-24 shrink-0 rounded-full" aria-hidden="true" />}
      {!isLoading && clientCount > 0 && (
        <Tag className="px-2.5 py-1">
          {clientCount} {t.clients.clientCount}
        </Tag>
      )}
    </div>
  );
}
