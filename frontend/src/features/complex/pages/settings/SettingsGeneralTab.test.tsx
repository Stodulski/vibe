import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SettingsGeneralTab } from './SettingsGeneralTab';
import type { Complex } from '@/shared/types/api.types';

vi.mock('@/features/complex', () => ({
  ComplexForm: () => <div data-testid="complex-form" />,
  ImageUpload: () => <div data-testid="image-upload" />,
  useDeleteComplex: () => ({ mutate: vi.fn(), isPending: false }),
}));

const complex = { id: 'c1', name: 'Club Vibe' } as Complex;

describe('SettingsGeneralTab', () => {
  it('opens the delete confirmation from the danger zone, with no overflow menu', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsGeneralTab complex={complex} />);

    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();
    const button = screen.getByRole('button', { name: ES_AR.complex.deleteComplex });

    await user.click(button);
    expect(await screen.findByRole('alertdialog')).toBeInTheDocument();
  });
});
