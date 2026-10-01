import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { ConnectAction } from './ConnectAction';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

describe('ConnectAction', () => {
  it('renders a disabled connect button, not nothing, while the auth URL is still being built', () => {
    render(<ConnectAction connected={false} authUrl={null} onConnectClick={vi.fn()} onDisconnectClick={vi.fn()} />);
    const button = screen.getByRole('button', { name: t.mp.connect });
    expect(button).toBeDisabled();
  });

  it('renders the connect link once the auth URL is ready', () => {
    render(
      <ConnectAction
        connected={false}
        authUrl="https://auth.mercadopago.com/test"
        onConnectClick={vi.fn()}
        onDisconnectClick={vi.fn()}
      />,
    );
    const link = screen.getByRole('link', { name: t.mp.connect });
    expect(link).toHaveAttribute('href', 'https://auth.mercadopago.com/test');
  });

  it('persists the attempt on click, middle-click and the context menu ("open in new tab")', () => {
    const onConnectClick = vi.fn();
    render(
      <ConnectAction
        connected={false}
        authUrl="https://auth.mercadopago.com/test"
        onConnectClick={onConnectClick}
        onDisconnectClick={vi.fn()}
      />,
    );
    const link = screen.getByRole('link', { name: t.mp.connect });

    fireEvent.click(link);
    fireEvent(link, new MouseEvent('auxclick', { bubbles: true, button: 1 }));
    fireEvent.contextMenu(link);

    expect(onConnectClick).toHaveBeenCalledTimes(3);
  });

  it('renders the disconnect button when connected, regardless of authUrl', () => {
    render(<ConnectAction connected authUrl={null} onConnectClick={vi.fn()} onDisconnectClick={vi.fn()} />);
    expect(screen.getByRole('button', { name: t.mp.disconnect })).toBeInTheDocument();
  });
});
