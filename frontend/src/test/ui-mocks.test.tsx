import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { passthrough } from './ui-mocks';

describe('passthrough', () => {
  it('renders the given tag and forwards children', () => {
    const Stub = passthrough('div');
    render(<Stub data-testid="stub">hello</Stub>);
    expect(screen.getByTestId('stub')).toHaveTextContent('hello');
  });

  it('spreads arbitrary props (e.g. onClick) onto the rendered tag', () => {
    const onClick = vi.fn();
    const Stub = passthrough('button');
    render(<Stub onClick={onClick}>click me</Stub>);
    fireEvent.click(screen.getByText('click me'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('renders a different tag (option) with its own attributes', () => {
    const Stub = passthrough('option');
    render(
      <select>
        <Stub value="v1">Option 1</Stub>
      </select>,
    );
    expect(screen.getByRole('option', { name: 'Option 1' })).toHaveValue('v1');
  });
});
