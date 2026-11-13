import { Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { Tag } from '@/shared/components/common/Tag';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ClientCountSkeleton } from './ClientsSearchBarSkeleton';
import { CLIENTS_SEARCH_BAR_CLASS } from './clientsSearchBarLayout';

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
    <div className={CLIENTS_SEARCH_BAR_CLASS}>
      <IconInput
        icon={Search}
        value={searchInput}
        onChange={(e) => {
          onSearchChange(e.target.value);
        }}
        placeholder={t.common.search}
        aria-label={`${t.common.search} clientes`}
      />
      {isLoading && <ClientCountSkeleton />}
      {!isLoading && clientCount > 0 && (
        <Tag className="px-2.5 py-1">
          {clientCount} {t.clients.clientCount}
        </Tag>
      )}
    </div>
  );
}
