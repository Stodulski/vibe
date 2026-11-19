import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DangerZone } from './DangerZone';

describe('DangerZone', () => {
  it('names the section, says what happens in one line and offers a visible destructive button', async () => {
    const onAction = vi.fn();
    render(<DangerZone description="Se borra todo." actionLabel="Eliminar cuenta" onAction={onAction} />);

    expect(screen.getByRole('heading', { name: ES_AR.common.dangerZone })).toBeInTheDocument();
    expect(screen.getByText('Se borra todo.')).toBeInTheDocument();
    const button = screen.getByRole('button', { name: 'Eliminar cuenta' });

    await userEvent.click(button);
    expect(onAction).toHaveBeenCalledOnce();
  });
});
