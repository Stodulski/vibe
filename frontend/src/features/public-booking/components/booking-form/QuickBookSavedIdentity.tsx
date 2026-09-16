import { Pencil, User } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import type { SavedClientData } from './savedFormData';

const t = ES_AR;

interface QuickBookSavedIdentityProps {
  saved: SavedClientData;
  onEdit: () => void;
}

/** The saved name/phone/email row at the top of `QuickBookView`'s panel, plus the small "Cambiar" edit action. */
export function QuickBookSavedIdentity({ saved, onEdit }: QuickBookSavedIdentityProps) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <div className="bg-primary-500/10 flex size-10 shrink-0 items-center justify-center rounded-full">
          <User className="text-primary-400 size-4" />
        </div>
        <div className="min-w-0">
          <p className="text-text-primary truncate text-sm font-medium">
            {saved.client_first_name} {saved.client_last_name}
          </p>
          <p className="text-text-tertiary truncate text-xs">
            {DEFAULT_PHONE_PREFIX} {saved.client_phone}
            {saved.client_email ? ` · ${saved.client_email}` : ''}
          </p>
        </div>
      </div>
      <button
        type="button"
        onClick={onEdit}
        className="text-text-tertiary hover:bg-bg-elevated hover:text-text-secondary flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
      >
        <Pencil className="size-3" />
        {t.common.change}
      </button>
    </div>
  );
}
