import type { ReactNode } from 'react';
import { XIcon } from 'lucide-react';
import { SheetHeader, SheetTitle, SheetClose } from '@/shared/components/ui/sheet';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface DetailSheetHeaderProps {
  title: string;
  /** Small line under the title (e.g. phone number). */
  subtitle?: ReactNode;
  /** Trailing action next to the close button — typically a kebab menu. */
  menu?: ReactNode;
  /** Second row below the title (status badges, tags, etc). */
  children?: ReactNode;
}

/**
 * Sheet header for detail drawers: title, an optional trailing menu, and a
 * close button all sharing one flex row so they stay vertically aligned —
 * the Sheet's own default close button is absolutely positioned and drifts
 * out of line with anything else in the header, so pair this with
 * `<SheetContent showCloseButton={false}>`.
 */
export function DetailSheetHeader({ title, subtitle, menu, children }: DetailSheetHeaderProps) {
  return (
    <SheetHeader>
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <SheetTitle className="truncate text-lg font-bold">{title}</SheetTitle>
          {/* Callers style the subtitle's own content — a phone number reads
              as a figure, a section name as a label. */}
          {subtitle && <p className="mt-0.5 truncate text-sm text-text-tertiary">{subtitle}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {menu}
          <SheetClose asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t.common.close}
              className="relative before:absolute before:-inset-2 before:content-['']"
            >
              <XIcon className="size-4" />
            </Button>
          </SheetClose>
        </div>
      </div>
      {children}
    </SheetHeader>
  );
}
