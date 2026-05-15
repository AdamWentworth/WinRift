import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import { App } from './App';

vi.mock('./api/client', () => ({
  getBuilds: vi.fn(async () => ({ results: [] })),
  getChampions: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getItems: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getLiveGame: vi.fn(),
}));

describe('App', () => {
  it('renders the core workspace', async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );

    expect(screen.getByText('WinRift')).toBeInTheDocument();
    expect(screen.getByText('Build Explorer')).toBeInTheDocument();
    expect(screen.getByText('Live Context')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('No build patterns match the current filters.')).toBeInTheDocument());
    expect(screen.queryByText(/buy this/i)).not.toBeInTheDocument();
    queryClient.clear();
  });
});
