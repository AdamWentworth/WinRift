import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { getLiveGame, getRunes, resolveAccountAlias, searchAccountAliases } from './api/client';
import type { LiveGame } from './api/types';

vi.mock('./api/client', () => ({
  getBuilds: vi.fn(async () => ({ results: [] })),
  getItemSlots: vi.fn(async () => ({ results: [] })),
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
    vi.mocked(resolveAccountAlias).mockReset();
    vi.mocked(searchAccountAliases).mockReset();
    vi.mocked(getRunes).mockReset();
    vi.mocked(resolveAccountAlias).mockResolvedValue({ status: 'not_found' });
    vi.mocked(searchAccountAliases).mockResolvedValue({ matches: [] });
    vi.mocked(getRunes).mockResolvedValue({ version: 'test', data: [] });
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

  it('shows full rune selections and stat shards in the rune popover', async () => {
    vi.mocked(getRunes).mockResolvedValueOnce({
      version: 'test',
      data: [
        {
          id: 8100,
          key: 'Domination',
          name: 'Domination',
          icon: 'perk-images/Styles/7200_Domination.png',
          slots: [
            { runes: [{ id: 8112, key: 'Electrocute', name: 'Electrocute', icon: 'perk-images/Styles/Domination/Electrocute/Electrocute.png' }] },
            { runes: [{ id: 8126, key: 'CheapShot', name: 'Cheap Shot', icon: 'perk-images/Styles/Domination/CheapShot/CheapShot.png' }] },
            { runes: [{ id: 8138, key: 'EyeballCollection', name: 'Eyeball Collection', icon: 'perk-images/Styles/Domination/EyeballCollection/EyeballCollection.png' }] },
            { runes: [{ id: 8106, key: 'UltimateHunter', name: 'Ultimate Hunter', icon: 'perk-images/Styles/Domination/UltimateHunter/UltimateHunter.png' }] },
          ],
        },
        {
          id: 8200,
          key: 'Sorcery',
          name: 'Sorcery',
          icon: 'perk-images/Styles/7202_Sorcery.png',
          slots: [
            { runes: [{ id: 8226, key: 'ManaflowBand', name: 'Manaflow Band', icon: 'perk-images/Styles/Sorcery/ManaflowBand/ManaflowBand.png' }] },
            { runes: [] },
            { runes: [{ id: 8237, key: 'Scorch', name: 'Scorch', icon: 'perk-images/Styles/Sorcery/Scorch/Scorch.png' }] },
          ],
        },
      ],
    });
    const liveGame: LiveGame = {
      platform: 'NA1',
      puuid: 'blue-puuid',
      gameId: 456,
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
          summonerName: 'Rune Blue',
          perks: {
            perkIds: [8112, 8126, 8138, 8106, 8226, 8237, 5008, 5008, 5002],
            perkStyle: 8100,
            perkSubStyle: 8200,
          },
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
      target: { value: 'Rune Blue#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Find live game'));

    const runeTrigger = await screen.findByLabelText('Rune selections');
    fireEvent.mouseEnter(runeTrigger);

    expect(await screen.findByText('Domination')).toBeInTheDocument();
    expect(screen.getByText('Electrocute')).toBeInTheDocument();
    expect(screen.getByText('Cheap Shot')).toBeInTheDocument();
    expect(screen.getByText('Eyeball Collection')).toBeInTheDocument();
    expect(screen.getByText('Ultimate Hunter')).toBeInTheDocument();
    expect(screen.getByText('Sorcery')).toBeInTheDocument();
    expect(screen.getByText('Manaflow Band')).toBeInTheDocument();
    expect(screen.getByText('Scorch')).toBeInTheDocument();
    expect(screen.getByText('Stat Shards')).toBeInTheDocument();
    expect(screen.getByText('Offense: Adaptive Force')).toBeInTheDocument();
    expect(screen.getByText('Flex: Adaptive Force')).toBeInTheDocument();
    expect(screen.getByText('Defense: Armor')).toBeInTheDocument();
    queryClient.clear();
  });
});
