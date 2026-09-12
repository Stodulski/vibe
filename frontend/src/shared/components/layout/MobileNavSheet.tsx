import type { ReactNode } from 'react';
import { Sheet, SheetContent, SheetTitle } from '@/shared/components/ui/sheet';
import { VisuallyHidden } from '@/shared/components/common/VisuallyHidden';
import { ES_AR } from '@/shared/i18n/es_AR';

interface MobileNavSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sidebar: ReactNode;
}

/** Mobile slide-in sheet that hosts a shell's sidebar content, shared between the owner dashboard and admin shells. */
export function MobileNavSheet({ open, onOpenChange, sidebar }: MobileNavSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="left"
        className="bg-bg-base border-border-subtle w-[280px] max-w-[85vw] gap-0 p-0"
        showCloseButton={false}
      >
        <VisuallyHidden>
          <SheetTitle>{ES_AR.layout.mobileMenuTitle}</SheetTitle>
        </VisuallyHidden>
        {/* No drag handle. A horizontal grabber is the affordance of a sheet
            you pull down, and this one enters from the side and leaves the
            same way — it never moved vertically, so the handle described a
            gesture the panel does not have. */}
        <div className="min-h-0 flex-1 pt-3">{sidebar}</div>
      </SheetContent>
    </Sheet>
  );
}
