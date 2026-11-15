import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BlockedSlotDetail } from './BlockedSlotDetail';
import type { BlockedSlot } from '@/shared/types/api.types';

const slot: BlockedSlot = {
  id: 's1',
  court_id: 'ct1',
  court_name: 'Cancha 1',
  date: '2099-01-06T00:00:00Z',
  start_time: '10:00',
  end_time: '11:00',
  reason: 'Torneo',
} as BlockedSlot;

function renderDetail(props: Partial<React.ComponentProps<typeof BlockedSlotDetail>> = {}) {
  const onDelete = vi.fn();
  renderWithProviders(<BlockedSlotDetail open onClose={vi.fn()} slot={slot} onDelete={onDelete} {...props} />);
  return { onDelete, user: userEvent.setup() };
}

describe('BlockedSlotDetail', () => {
  it('keeps "Desbloquear" in the header menu as a destructive item, with no footer', async () => {
    const { user, onDelete } = renderDetail();

    expect(screen.queryByRole('button', { name: ES_AR.courts.unblockSlot })).not.toBeInTheDocument();
    expect(document.querySelector('[data-slot="sheet-footer"]')).toBeNull();

    await user.click(screen.getByRole('button', { name: ES_AR.common.rowActionsLabel }));
    const item = await screen.findByRole('menuitem', { name: ES_AR.courts.unblockSlot });
    expect(item).toHaveAttribute('data-variant', 'destructive');

    await user.click(item);
    expect(onDelete).toHaveBeenCalledWith('s1');
  });
});
