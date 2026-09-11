import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { LayoutDashboard, CalendarDays, Trophy, Users, FileBarChart, Settings } from 'lucide-react';
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from '@/shared/components/ui/command';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const NAV_ITEMS = [
  { to: '/dashboard', label: t.navigation.dashboard, icon: LayoutDashboard },
  { to: '/bookings', label: t.navigation.bookings, icon: CalendarDays },
  { to: '/courts', label: t.navigation.courts, icon: Trophy },
  { to: '/clients', label: t.navigation.clients, icon: Users },
  { to: '/reports', label: t.navigation.reports, icon: FileBarChart },
  { to: '/settings', label: t.navigation.settings, icon: Settings },
];

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setOpen((prev) => !prev);
      }
    }

    function handleCustomOpen() {
      setOpen(true);
    }

    document.addEventListener('keydown', handleKeyDown);
    window.addEventListener('open-command-palette', handleCustomOpen);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('open-command-palette', handleCustomOpen);
    };
  }, []);

  const runCommand = useCallback((command: () => void) => {
    setOpen(false);
    command();
  }, []);

  return (
    <CommandDialog
      open={open}
      onOpenChange={setOpen}
      title={t.commandPalette.title}
      description={t.commandPalette.description}
      showCloseButton={false}
    >
      <CommandInput placeholder={t.commandPalette.placeholder} />
      <CommandList>
        <CommandEmpty>{t.common.noResults}</CommandEmpty>
        <CommandGroup heading={t.commandPalette.navigation}>
          {NAV_ITEMS.map((item) => (
            <CommandItem
              key={item.to}
              onSelect={() => {
                runCommand(() => {
                  void navigate(item.to);
                });
              }}
            >
              <item.icon className="size-4" />
              {item.label}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
