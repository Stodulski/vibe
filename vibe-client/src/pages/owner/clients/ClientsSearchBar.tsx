import { Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { Tag } from '@/shared/components/common/Tag';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ClientsSearchBarProps {
  searchInput: string;
  onSearchChange: (value: string) => void;
  clientCount: number;
}

export function ClientsSearchBar({ searchInput, onSearchChange, clientCount }: ClientsSearchBarProps) {
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
      {clientCount > 0 && (
        <Tag className="px-2.5 py-1">
          {clientCount} {t.clients.clientCount}
        </Tag>
      )}
    </div>
  );
}
