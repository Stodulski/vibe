import { Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { roleOptions } from './userTableUtils';

const t = ES_AR;

interface UsersTableFiltersProps {
  searchInput: string;
  onSearchChange: (value: string) => void;
  roleFilter: string;
  onRoleFilterChange: (value: string) => void;
}

export function UsersTableFilters({
  searchInput,
  onSearchChange,
  roleFilter,
  onRoleFilterChange,
}: UsersTableFiltersProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
      <IconInput
        icon={Search}
        placeholder={t.admin.users.searchPlaceholder}
        value={searchInput}
        onChange={(e) => {
          onSearchChange(e.target.value);
        }}
      />
      <Select value={roleFilter} onValueChange={onRoleFilterChange}>
        <SelectTrigger className="w-full sm:w-[180px]">
          <SelectValue placeholder={t.admin.users.roleFilter} />
        </SelectTrigger>
        <SelectContent>
          {roleOptions.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {opt.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
