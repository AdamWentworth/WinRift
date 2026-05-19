import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { getChampionRoleRates, getLiveGame, resolveAccountAlias, searchAccountAliases } from './api/client';
import type { LiveGame } from './api/types';

vi.mock('./api/client', () => ({
  getBuilds: vi.fn(async () => ({ results: [] })),
  getItemSlots: vi.fn(async () => ({ results: [] })),
  getChampionRoleRates: vi.fn(async () => ({ results: [] })),
  getChampions: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getItems: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getRunes: vi.fn(async () => ({ version: 'test', data: [] })),
  getSummonerSpells: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getLiveGame: vi.fn(),
  resolveAccountAlias: vi.fn(async () => ({ status: 'not_found' })),
  searchAccountAliases: vi.fn(async () => ({ matches: [] })),
}));

describe('App', () => {
  afterEach(() => {
    cleanup();
    vi.mocked(getLiveGame).mockReset();
    vi.mocked(getChampionRoleRates).mockReset();
    vi.mocked(resolveAccountAlias).mockReset();
    vi.mocked(searchAccountAliases).mockReset();
    vi.mocked(getChampionRoleRates).mockResolvedValue({ results: [] });
    vi.mocked(resolveAccountAlias).mockResolvedValue({ status: 'not_found' });
    vi.mocked(searchAccountAliases).mockResolvedValue({ matches: [] });
  });

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
    expect(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('Build Explorer')).not.toBeInTheDocument());
    expect(screen.queryByText('Contextual Patterns')).not.toBeInTheDocument();
    expect(screen.queryByText(/buy this/i)).not.toBeInTheDocument();
    queryClient.clear();
  });

  it('shows the legacy live-match miss message', async () => {
    vi.mocked(getLiveGame).mockRejectedValueOnce(new Error('Player is not currently in a live game'));
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

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'Test Summoner#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    await waitFor(() => expect(screen.getByText("Summoner 'Test Summoner#NA1' is not currently in a live match")).toBeInTheDocument());
    queryClient.clear();
  });

  it('requires a unique saved Riot ID before calling the live API without a tag', async () => {
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

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'Test Summoner' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    await waitFor(() => expect(screen.getByText("Tag required. No unique saved Riot ID for 'Test Summoner'. Use Name#Tag.")).toBeInTheDocument());
    expect(resolveAccountAlias).toHaveBeenCalledWith('Test Summoner', 'NA1');
    expect(getLiveGame).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it('uses a unique saved Riot ID alias for tagless lookup', async () => {
    vi.mocked(resolveAccountAlias).mockResolvedValueOnce({
      status: 'found',
      puuid: 'puuid',
      platform: 'NA1',
      gameName: 'Sneaky',
      tagLine: 'NA69',
    });
    vi.mocked(getLiveGame).mockRejectedValueOnce(new Error('Player is not currently in a live game'));
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

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'sneaky' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    await waitFor(() => expect(getLiveGame).toHaveBeenCalledWith('Sneaky', 'NA69', 'NA1'));
    await waitFor(() => expect(screen.getByDisplayValue('Sneaky#NA69')).toBeInTheDocument());
    queryClient.clear();
  });

  it('shows saved Riot ID autocomplete matches', async () => {
    vi.mocked(searchAccountAliases).mockResolvedValueOnce({
      matches: [
        {
          puuid: 'puuid',
          platform: 'NA1',
          gameName: 'Sneaky',
          tagLine: 'NA69',
        },
      ],
    });
    vi.mocked(getLiveGame).mockRejectedValueOnce(new Error('Player is not currently in a live game'));
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

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'Sne' },
    });

    await waitFor(() => expect(searchAccountAliases).toHaveBeenCalledWith('Sne', 'NA1', 6));
    fireEvent.click(await screen.findByLabelText('Use Sneaky#NA69'));

    await waitFor(() => expect(getLiveGame).toHaveBeenCalledWith('Sneaky', 'NA69', 'NA1'));
    expect(screen.getByDisplayValue('Sneaky#NA69')).toBeInTheDocument();
    queryClient.clear();
  });

  it('shows ranked record stats on live player cards', async () => {
    const liveGame: LiveGame = {
      platform: 'NA1',
      puuid: 'blue-puuid',
      gameId: 123,
      mapId: 11,
      gameMode: 'CLASSIC',
      gameType: 'MATCHED_GAME',
      gameQueueConfigId: 420,
      gameStartTime: Date.now(),
      participants: [
        {
          teamId: 100,
          championId: 1,
          spell1Id: 4,
          spell2Id: 14,
          puuid: 'blue-puuid',
          summonerName: 'Ranked Blue',
          perks: { perkIds: [8112], perkSubStyle: 8000 },
          bot: false,
          rank: {
            queueType: 'RANKED_SOLO_5x5',
            tier: 'DIAMOND',
            division: 'II',
            leaguePoints: 44,
            wins: 30,
            losses: 20,
            totalGames: 50,
            winRate: 60,
            rankBucket: 'DIAMOND',
          },
          championStats: {
            queueId: 420,
            championId: 1,
            games: 30,
            wins: 20,
            losses: 10,
            kills: 210,
            deaths: 90,
            assists: 195,
            avgKills: 7,
            avgDeaths: 3,
            avgAssists: 6.5,
            kda: 4.5,
            winRate: 66.67,
          },
        },
        {
          teamId: 200,
          championId: 2,
          spell1Id: 4,
          spell2Id: 12,
          puuid: 'red-puuid',
          summonerName: 'Ranked Red',
          perks: { perkIds: [8005], perkSubStyle: 8100 },
          bot: false,
        },
      ],
    };
    vi.mocked(getLiveGame).mockResolvedValueOnce(liveGame);
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

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'Ranked Blue#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    await waitFor(() => expect(screen.getByText('DIAMOND II')).toBeInTheDocument());
    expect(screen.getByText('Winrate: 60.0%')).toBeInTheDocument();
    expect(screen.getByText('Games: 50')).toBeInTheDocument();
    expect(screen.getByText('Champ WR: 66.7%')).toBeInTheDocument();
    expect(screen.getByText('KDA: 4.50')).toBeInTheDocument();
    expect(screen.getByText('Avg: 7.0 / 3.0 / 6.5')).toBeInTheDocument();
    queryClient.clear();
  });

  it('uses smite and champion role rates to place live cards by role', async () => {
    vi.mocked(getChampionRoleRates).mockResolvedValueOnce({
      results: [
        { championId: 1, role: 'TOP', games: 75, totalGames: 100, pickRate: 75 },
        { championId: 1, role: 'UTILITY', games: 25, totalGames: 100, pickRate: 25 },
        { championId: 2, role: 'JUNGLE', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 3, role: 'MIDDLE', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 4, role: 'BOTTOM', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 5, role: 'UTILITY', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 11, role: 'TOP', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 12, role: 'JUNGLE', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 13, role: 'MIDDLE', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 14, role: 'BOTTOM', games: 100, totalGames: 100, pickRate: 100 },
        { championId: 15, role: 'UTILITY', games: 100, totalGames: 100, pickRate: 100 },
      ],
    });
    vi.mocked(getLiveGame).mockResolvedValueOnce({
      platform: 'NA1',
      puuid: 'blue-puuid',
      gameId: 321,
      mapId: 11,
      gameMode: 'CLASSIC',
      gameType: 'MATCHED_GAME',
      gameQueueConfigId: 420,
      gameStartTime: Date.now(),
      participants: [
        { teamId: 100, championId: 4, spell1Id: 4, spell2Id: 7, puuid: 'blue-bot', summonerName: 'Blue Bot', perks: {}, bot: false },
        { teamId: 100, championId: 5, spell1Id: 4, spell2Id: 14, puuid: 'blue-support', summonerName: 'Blue Support', perks: {}, bot: false },
        { teamId: 100, championId: 3, spell1Id: 4, spell2Id: 12, puuid: 'blue-mid', summonerName: 'Blue Mid', perks: {}, bot: false },
        { teamId: 100, championId: 1, spell1Id: 4, spell2Id: 12, puuid: 'blue-top', summonerName: 'Blue Top', perks: {}, bot: false },
        { teamId: 100, championId: 2, spell1Id: 11, spell2Id: 4, puuid: 'blue-jungle', summonerName: 'Blue Jungle', perks: {}, bot: false },
        { teamId: 200, championId: 14, spell1Id: 4, spell2Id: 7, puuid: 'red-bot', summonerName: 'Red Bot', perks: {}, bot: false },
        { teamId: 200, championId: 15, spell1Id: 4, spell2Id: 14, puuid: 'red-support', summonerName: 'Red Support', perks: {}, bot: false },
        { teamId: 200, championId: 13, spell1Id: 4, spell2Id: 12, puuid: 'red-mid', summonerName: 'Red Mid', perks: {}, bot: false },
        { teamId: 200, championId: 11, spell1Id: 4, spell2Id: 12, puuid: 'red-top', summonerName: 'Red Top', perks: {}, bot: false },
        { teamId: 200, championId: 12, spell1Id: 11, spell2Id: 4, puuid: 'red-jungle', summonerName: 'Red Jungle', perks: {}, bot: false },
      ],
    });
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByPlaceholderText('Riot ID, e.g. TWITCH ELOSANTA#1111'), {
      target: { value: 'Blue Top#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    await waitFor(() => expect(getChampionRoleRates).toHaveBeenCalledWith([1, 2, 3, 4, 5, 11, 12, 13, 14, 15], 420));
    await waitFor(() => {
      expect([...container.querySelectorAll('.blue-row .summoner-name')].map((node) => node.textContent)).toEqual([
        'Blue Top',
        'Blue Jungle',
        'Blue Mid',
        'Blue Bot',
        'Blue Support',
      ]);
      expect([...container.querySelectorAll('.red-row .summoner-name')].map((node) => node.textContent)).toEqual([
        'Red Top',
        'Red Jungle',
        'Red Mid',
        'Red Bot',
        'Red Support',
      ]);
    });
    queryClient.clear();
  });
});
