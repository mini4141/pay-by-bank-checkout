import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import Timeline from './Timeline.svelte';

describe('Timeline', () => {
  it('renders every step and marks completed ones', () => {
    render(Timeline, {
      props: {
        events: [
          { step: 'RESERVE', status: 'OK' },
          { step: 'RISK', status: 'OK' },
          { step: 'DEBIT', status: 'RETRY' }
        ]
      }
    });
    expect(screen.getByText('Funds reserved')).toBeTruthy();
    expect(screen.getByText('Debited')).toBeTruthy();
    expect(screen.getByText('Settled')).toBeTruthy();
    expect(screen.getAllByText('complete')).toHaveLength(2);
    expect(screen.getByText('retrying')).toBeTruthy();
  });
});
