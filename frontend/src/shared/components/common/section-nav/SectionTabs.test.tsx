import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LayoutDashboard, Settings } from 'lucide-react';
import { SectionTabs } from './SectionTabs';
import type { SectionNavItem } from './SectionNav';

const items: SectionNavItem<'general' | 'security'>[] = [
  { value: 'general', label: 'General', icon: LayoutDashboard },
  { value: 'security', label: 'Seguridad', icon: Settings },
];

describe('SectionTabs', () => {
  it('marks the active tab with aria-pressed instead of aria-current', () => {
    render(<SectionTabs items={items} active="security" onChange={vi.fn()} />);

    const activeButton = screen.getByRole('button', { name: 'Seguridad' });
    const inactiveButton = screen.getByRole('button', { name: 'General' });

    expect(activeButton).toHaveAttribute('aria-pressed', 'true');
    expect(inactiveButton).toHaveAttribute('aria-pressed', 'false');
    // These are section-switching buttons, not links to another page.
    expect(activeButton).not.toHaveAttribute('aria-current');
  });

  it('calls onChange with the clicked section value', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SectionTabs items={items} active="general" onChange={onChange} />);

    await user.click(screen.getByRole('button', { name: 'Seguridad' }));

    expect(onChange).toHaveBeenCalledWith('security');
  });
});
