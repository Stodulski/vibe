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
  it('shows "Desbloquear" as a destructive outline button in the footer, with no overflow menu', async () => {
    const { user, onDelete } = renderDetail();

    const button = screen.getByRole('button', { name: ES_AR.courts.unblockSlot });
    expect(button).toHaveAttribute('data-variant', 'destructive-outline');
    expect(button.closest('[data-slot="sheet-footer"]')).not.toBeNull();
    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();

    await user.click(button);
    expect(onDelete).toHaveBeenCalledWith('s1');
  });

  it('disables the button while the slot is being unblocked', () => {
    renderDetail({ isDeleting: true });

    expect(screen.getByRole('button', { name: ES_AR.courts.unblockSlot })).toBeDisabled();
  });
});
