import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { getBuildAdvice, getChampionGuide, getChampionGuideIndex, getChampionRoleRates, getLiveGame, getSummonerProfile, getWinConditionAnalysis, resolveAccountAlias, searchAccountAliases } from './api/client';
import type { LiveGame } from './api/types';

const buildAdviceFixture = vi.hoisted(() => () => ({
  filters: {
    championId: 3,
    role: 'MIDDLE',
    opponentChampionId: 13,
    patch: '16.10',
    rankBucket: '',
    itemContext: '',
    minGames: 5,
    championMinGames: 10,
    limit: 12,
  },
  matchup: {
    available: true,
    itemSlots: [
      { championId: 3, role: 'MIDDLE', opponentChampionId: 13, patchBucket: 'ALL', rankBucket: 'ALL', itemSlot: 1, itemId: 100, wins: 6, games: 10, winRate: 60, confidence: 45, sampleScopeLabel: 'Current patch exact matchup' },
      { championId: 3, role: 'MIDDLE', opponentChampionId: 13, patchBucket: 'ALL', rankBucket: 'ALL', itemSlot: 1, itemId: 101, wins: 27, games: 50, winRate: 54, confidence: 48, sampleScopeLabel: 'Current patch exact matchup' },
    ],
    topBuilds: [],
    sample: { maxGames: 50, optionCount: 2, fallbackUsed: false, scopeLabels: ['Current patch exact matchup'], sampleQuality: 'moderate', sampleQualityLabel: 'Moderate sample' },
    sampleMode: 'champion_matchup',
  },
  champion: {
    available: true,
    itemSlots: [
      { championId: 3, role: 'MIDDLE', opponentChampionId: 0, patchBucket: 'ALL', rankBucket: 'ALL', itemSlot: 1, itemId: 100, wins: 10, games: 20, winRate: 50, confidence: 40, sampleScopeLabel: 'Current patch champion overall' },
    ],
    topBuilds: [],
    topRunes: [],
    topSpells: [],
    topItemPaths: [],
    buildVariants: [],
    summary: { championId: 3, role: 'MIDDLE', patchBucket: '16.10', rankBucket: 'ALL', wins: 12, games: 20, bans: 3, winRate: 60, confidence: 40, pickRate: 2.2, banRate: 1.1, roleRank: 1, roleRankTotal: 12 },
    sample: { maxGames: 20, optionCount: 1, fallbackUsed: false, scopeLabels: ['Current patch champion overall'], sampleQuality: 'early', sampleQualityLabel: 'Early sample' },
    sampleMode: 'champion_overall',
    strictRoleUsed: true,
  },
  notes: ['Build advice is compiled from stored ranked Solo/Duo games and refreshed summary tables.'],
}));

vi.mock('./api/client', () => ({
  getBuilds: vi.fn(async () => ({ results: [] })),
  getBuildAdvice: vi.fn(async () => buildAdviceFixture()),
  getChampionGuide: vi.fn(async () => ({
    summary: { championId: 62, role: 'JUNGLE', patchBucket: '16.10', rankBucket: 'ALL', wins: 12, games: 20, bans: 3, winRate: 60, confidence: 40, pickRate: 2.2, banRate: 1.1, roleRank: 1, roleRankTotal: 12 },
    toughestMatchups: [],
    bestMatchups: [],
    topRunes: [],
    topSpells: [],
    topSkillOrders: [],
    topItemPaths: [],
    buildVariants: [],
  })),
  getChampionGuideIndex: vi.fn(async () => ({
    results: [
      { championId: 62, role: 'JUNGLE', patchBucket: '16.10', rankBucket: 'ALL', wins: 12, games: 20, bans: 3, winRate: 60, confidence: 40, pickRate: 2.2, banRate: 1.1, roleRank: 1, roleRankTotal: 12 },
    ],
  })),
  getItemSlots: vi.fn(async () => ({ results: [] })),
  getChampionRoleRates: vi.fn(async () => ({ results: [] })),
  getWinConditionAnalysis: vi.fn(async () => winConditionFixture),
  getChampions: vi.fn(async () => ({
    version: '16.10.1',
    data: {
      data: {
        MonkeyKing: {
          id: 'MonkeyKing',
          key: '62',
          name: 'Wukong',
          title: 'the Monkey King',
          image: { full: 'MonkeyKing.png' },
          passive: { id: 'MonkeyKingPassive', name: 'Stone Skin', image: { full: 'MonkeyKingStoneSkin.png' } },
          spells: [
            { id: 'MonkeyKingQ', name: 'Crushing Blow', image: { full: 'MonkeyKingQ.png' } },
            { id: 'MonkeyKingW', name: 'Warrior Trickster', image: { full: 'MonkeyKingW.png' } },
            { id: 'MonkeyKingE', name: 'Nimbus Strike', image: { full: 'MonkeyKingE.png' } },
            { id: 'MonkeyKingR', name: 'Cyclone', image: { full: 'MonkeyKingR.png' } },
          ],
        },
      },
    },
  })),
  getChampionSplashes: vi.fn(async () => ({ version: 'test', data: [] })),
  getItems: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getRunes: vi.fn(async () => ({ version: 'test', data: [] })),
  getSummonerSpells: vi.fn(async () => ({ version: 'test', data: { data: {} } })),
  getLiveGame: vi.fn(),
  getSummonerProfile: vi.fn(async () => ({
    account: { puuid: 'profile-puuid', platform: 'NA1', gameName: 'Test Summoner', tagLine: 'NA1' },
    summary: { puuid: 'profile-puuid', platform: 'NA1', queueId: 420, games: 0, wins: 0, losses: 0, kills: 0, deaths: 0, assists: 0, avgKills: 0, avgDeaths: 0, avgAssists: 0, kda: 0, winRate: 0 },
    topChampions: [],
    topBuilds: [],
    recentMatches: [],
  })),
  resolveAccountAlias: vi.fn(async () => ({ status: 'not_found' })),
  searchAccountAliases: vi.fn(async () => ({ matches: [] })),
}));

describe('App', () => {
  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
    vi.mocked(getLiveGame).mockReset();
    vi.mocked(getSummonerProfile).mockReset();
    vi.mocked(getChampionRoleRates).mockReset();
    vi.mocked(getChampionGuide).mockReset();
    vi.mocked(getChampionGuideIndex).mockReset();
    vi.mocked(getBuildAdvice).mockReset();
    vi.mocked(getWinConditionAnalysis).mockReset();
    vi.mocked(resolveAccountAlias).mockReset();
    vi.mocked(searchAccountAliases).mockReset();
    vi.mocked(getLiveGame).mockRejectedValue(new Error('Player is not currently in a live game'));
    vi.mocked(getChampionRoleRates).mockResolvedValue({ results: [] });
    vi.mocked(getChampionGuide).mockResolvedValue({
      summary: { championId: 62, role: 'JUNGLE', patchBucket: '16.10', rankBucket: 'ALL', wins: 12, games: 20, bans: 3, winRate: 60, confidence: 40, pickRate: 2.2, banRate: 1.1, roleRank: 1, roleRankTotal: 12 },
      toughestMatchups: [],
      bestMatchups: [],
      topRunes: [],
      topSpells: [],
      topSkillOrders: [],
      topItemPaths: [],
      buildVariants: [],
    });
    vi.mocked(getChampionGuideIndex).mockResolvedValue({
      results: [
        { championId: 62, role: 'JUNGLE', patchBucket: '16.10', rankBucket: 'ALL', wins: 12, games: 20, bans: 3, winRate: 60, confidence: 40, pickRate: 2.2, banRate: 1.1, roleRank: 1, roleRankTotal: 12 },
      ],
    });
    vi.mocked(getBuildAdvice).mockResolvedValue(buildAdviceFixture());
    vi.mocked(getWinConditionAnalysis).mockResolvedValue(winConditionFixture);
    vi.mocked(getSummonerProfile).mockResolvedValue({
      account: { puuid: 'profile-puuid', platform: 'NA1', gameName: 'Test Summoner', tagLine: 'NA1' },
      summary: { puuid: 'profile-puuid', platform: 'NA1', queueId: 420, games: 0, wins: 0, losses: 0, kills: 0, deaths: 0, assists: 0, avgKills: 0, avgDeaths: 0, avgAssists: 0, kda: 0, winRate: 0 },
      topChampions: [],
      topBuilds: [],
      recentMatches: [],
    });
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
    expect(screen.getByText('Guides, Profiles, Live Games')).toBeInTheDocument();
    expect(screen.getByText('Champion names open build guides. Riot IDs open summoner profiles and jump into live match analysis when that player is currently in game.')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111')).toBeInTheDocument();
    expect(screen.queryByLabelText('Search champions or summoners')).not.toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('Build Explorer')).not.toBeInTheDocument());
    expect(screen.queryByText('Contextual Patterns')).not.toBeInTheDocument();
    expect(screen.queryByText(/buy this/i)).not.toBeInTheDocument();
    queryClient.clear();
  });

  it('opens the alphabetical champion directory and links into build guides', async () => {
    vi.mocked(getChampionRoleRates).mockResolvedValueOnce({
      results: [
        { championId: 62, role: 'JUNGLE', games: 120, totalGames: 140, pickRate: 85.71 },
        { championId: 62, role: 'MIDDLE', games: 20, totalGames: 140, pickRate: 14.29 },
      ],
    });
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

    fireEvent.click(screen.getByRole('button', { name: 'Champions' }));

    await waitFor(() => expect(screen.getByText('Champion Index')).toBeInTheDocument());
    expect(screen.getByText('All Champions')).toBeInTheDocument();
    expect(screen.queryByLabelText('Champion directory role')).not.toBeInTheDocument();
    expect(await screen.findByText('Wukong')).toBeInTheDocument();
    expect(screen.getByText('1 champions shown')).toBeInTheDocument();
    expect(screen.queryByText('60.0%')).not.toBeInTheDocument();
    expect(getChampionGuideIndex).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /Wukong/i }));
    await waitFor(() => expect(screen.getByText('WinRift Build Atlas')).toBeInTheDocument());
    expect(screen.getByText('Current Sample')).toBeInTheDocument();
    await waitFor(() => expect(getChampionGuide).toHaveBeenCalledWith(expect.objectContaining({ championId: 62, role: 'JUNGLE', patch: '16.10' })));
    expect(getChampionGuideIndex).toHaveBeenCalledWith(expect.objectContaining({ role: 'JUNGLE', patch: '16.10' }));
    queryClient.clear();
  });

  it('opens the role tier list and links into champion guides', async () => {
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

    fireEvent.click(screen.getByRole('button', { name: 'Tier List' }));

    await waitFor(() => expect(screen.getByText('WinRift Tier List')).toBeInTheDocument());
    expect(screen.getByText('All Roles Rankings')).toBeInTheDocument();
    expect(screen.getByLabelText('Tier list role')).toBeInTheDocument();
    await waitFor(() => expect(getChampionGuideIndex).toHaveBeenCalledWith(expect.objectContaining({ role: '', patch: '16.10', minGames: 50 })));
    expect((await screen.findAllByText('Wukong')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('60.00%').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Sort by Games, not sorted' }));
    expect(screen.getByText(/Games descending/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sort by Games, descending' }));
    expect(screen.getByText(/Games ascending/)).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: /Wukong/i })[0]);
    await waitFor(() => expect(screen.getByText('WinRift Build Atlas')).toBeInTheDocument());
    expect(window.location.pathname).toBe('/champions/MonkeyKing');
    queryClient.clear();
  });

  it('routes champion names from the homepage search to champion pages', async () => {
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

    await waitFor(() => expect(screen.getByTitle('Wukong')).toBeInTheDocument());
    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'wukong' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(screen.getByText('WinRift Build Atlas')).toBeInTheDocument());
    expect(window.location.pathname).toBe('/champions/MonkeyKing');
    expect(getLiveGame).not.toHaveBeenCalled();
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Test Summoner#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Test Summoner' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'sneaky' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(getLiveGame).toHaveBeenCalledWith('Sneaky', 'NA69', 'NA1'));
    await waitFor(() => expect(screen.getByText('Sneaky#NA69')).toBeInTheDocument());
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Sne' },
    });

    await waitFor(() => expect(searchAccountAliases).toHaveBeenCalledWith('Sne', 'NA1', 6));
    fireEvent.click(await screen.findByLabelText('Use Sneaky#NA69'));

    await waitFor(() => expect(getLiveGame).toHaveBeenCalledWith('Sneaky', 'NA69', 'NA1'));
    expect(screen.getByText('Sneaky#NA69')).toBeInTheDocument();
    queryClient.clear();
  });

  it('shows summoner-owned build paths on the profile page', async () => {
    vi.mocked(getSummonerProfile).mockResolvedValueOnce({
      account: { puuid: 'profile-puuid', platform: 'NA1', gameName: 'Profile Test', tagLine: 'NA1' },
      summary: { puuid: 'profile-puuid', platform: 'NA1', queueId: 420, games: 3, wins: 2, losses: 1, kills: 18, deaths: 9, assists: 21, avgKills: 6, avgDeaths: 3, avgAssists: 7, kda: 4.33, winRate: 66.67 },
      topChampions: [],
      recentMatches: [],
      topBuilds: [
        {
          platform: 'NA1',
          queueId: 420,
          championId: 62,
          role: 'JUNGLE',
          finalItemsSignature: '100-101-102-103-104-105',
          core2Signature: '100-101',
          core3Signature: '100-101-102',
          runeSignature: '',
          spellSignature: '4-11',
          games: 3,
          wins: 2,
          losses: 1,
          kills: 18,
          deaths: 9,
          assists: 21,
          avgKills: 6,
          avgDeaths: 3,
          avgAssists: 7,
          kda: 4.33,
          winRate: 66.67,
        },
      ],
    });
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Profile Test#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(screen.getByText('Profile Test#NA1')).toBeInTheDocument());
    await screen.findByText('Stored Form');
    fireEvent.click(screen.getByRole('button', { name: /^Builds$/i }));
    expect(await screen.findByText('This is usage history from stored ranked games for this summoner, not generalized build advice.')).toBeInTheDocument();
    expect(screen.getAllByText('Wukong').length).toBeGreaterThan(0);
    expect(screen.getAllByText('66.7%').length).toBeGreaterThan(0);
    expect(screen.getByText('4.33')).toBeInTheDocument();
    queryClient.clear();
  });

  it('filters summoner champion comfort by role', async () => {
    vi.mocked(getSummonerProfile).mockResolvedValueOnce({
      account: { puuid: 'profile-puuid', platform: 'NA1', gameName: 'Role Test', tagLine: 'NA1' },
      summary: { puuid: 'profile-puuid', platform: 'NA1', queueId: 420, games: 10, wins: 6, losses: 4, kills: 60, deaths: 30, assists: 70, avgKills: 6, avgDeaths: 3, avgAssists: 7, kda: 4.33, winRate: 60 },
      topChampions: [
        { queueId: 420, championId: 62, games: 10, wins: 6, losses: 4, kills: 60, deaths: 30, assists: 70, avgKills: 6, avgDeaths: 3, avgAssists: 7, kda: 4.33, winRate: 60 },
      ],
      topChampionRoles: [
        { queueId: 420, championId: 62, role: 'JUNGLE', games: 7, wins: 5, losses: 2, kills: 45, deaths: 20, assists: 50, avgKills: 6.4, avgDeaths: 2.9, avgAssists: 7.1, kda: 4.75, winRate: 71.43 },
      ],
      recentMatches: [],
      topBuilds: [],
    });
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Role Test#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(screen.getByText('Role Test#NA1')).toBeInTheDocument());
    await screen.findByText('Stored Form');
    fireEvent.click(screen.getByRole('button', { name: 'Champion Stats' }));
    fireEvent.click(screen.getByRole('button', { name: /Jungle/i }));

    expect(await screen.findByText('71.4%')).toBeInTheDocument();
    expect(screen.getAllByText('7').length).toBeGreaterThan(0);
    expect(screen.getByText(/Jungle · 6\.4 \/ 2\.9 \/ 7\.1 average/)).toBeInTheDocument();
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Ranked Blue#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(screen.getByText('DIAMOND II')).toBeInTheDocument());
    expect(screen.getByText('60.0%')).toBeInTheDocument();
    expect(screen.getByText('50')).toBeInTheDocument();
    expect(screen.getByText('66.7%')).toBeInTheDocument();
    expect(screen.getByText('4.50')).toBeInTheDocument();
    expect(screen.getByText('7.0 / 3.0 / 6.5')).toBeInTheDocument();
    queryClient.clear();
  });

  it('uses the WinRift logo to return from live match view to home search', async () => {
    vi.mocked(getLiveGame).mockResolvedValueOnce({
      platform: 'NA1',
      puuid: 'blue-puuid',
      gameId: 456,
      mapId: 11,
      gameMode: 'CLASSIC',
      gameType: 'MATCHED_GAME',
      gameQueueConfigId: 420,
      gameStartTime: Date.now(),
      participants: [
        { teamId: 100, championId: 1, spell1Id: 4, spell2Id: 14, puuid: 'blue-puuid', summonerName: 'Logo Blue', perks: {}, bot: false },
        { teamId: 200, championId: 2, spell1Id: 4, spell2Id: 12, puuid: 'red-puuid', summonerName: 'Logo Red', perks: {}, bot: false },
      ],
    });
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Logo Blue#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => expect(screen.getAllByText('Logo Blue').length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole('button', { name: 'WinRift home' }));

    await waitFor(() => expect(screen.queryByText('Logo Blue')).not.toBeInTheDocument());
    expect(screen.getByText('Guides, Profiles, Live Games')).toBeInTheDocument();
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Blue Top#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

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

  it('focuses build mode on the searched player after role sorting', async () => {
    vi.mocked(getChampionRoleRates).mockResolvedValueOnce({
      results: [
        { championId: 1, role: 'TOP', games: 100, totalGames: 100, pickRate: 100 },
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
      puuid: 'blue-mid',
      gameId: 322,
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Blue Mid#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    await waitFor(() => {
      expect([...container.querySelectorAll('.blue-row .summoner-name')].map((node) => node.textContent)).toEqual([
        'Blue Top',
        'Blue Jungle',
        'Blue Mid',
        'Blue Bot',
        'Blue Support',
      ]);
    });
    fireEvent.click(screen.getByRole('button', { name: 'Show Builds mode' }));

    await waitFor(() => {
      expect(getBuildAdvice).toHaveBeenLastCalledWith(expect.objectContaining({
        championId: 3,
        role: 'MIDDLE',
        opponentChampionId: 13,
        minGames: 5,
        championMinGames: 10,
        limit: 12,
        patch: '16.10',
      }));
      const calls = vi.mocked(getBuildAdvice).mock.calls;
      expect(calls[calls.length - 1]?.[0]).not.toHaveProperty('fallback');
    });
    expect(screen.getAllByText('Blue Mid').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Red Mid').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByLabelText('Build For Red Jungle'));
    await waitFor(() => {
      expect(getBuildAdvice).toHaveBeenLastCalledWith(expect.objectContaining({
        championId: 12,
        role: 'JUNGLE',
        opponentChampionId: 2,
        itemContext: 'JUNGLE',
      }));
    });
    queryClient.clear();
  });

  it('shows live win condition stats', async () => {
    vi.mocked(getLiveGame).mockResolvedValueOnce({
      platform: 'NA1',
      puuid: 'blue-1',
      gameId: 654,
      mapId: 11,
      gameMode: 'CLASSIC',
      gameType: 'MATCHED_GAME',
      gameQueueConfigId: 420,
      gameStartTime: Date.now(),
      participants: [
        { teamId: 100, championId: 1, spell1Id: 4, spell2Id: 12, puuid: 'blue-1', summonerName: 'Blue 1', perks: {}, bot: false },
        { teamId: 100, championId: 2, spell1Id: 11, spell2Id: 4, puuid: 'blue-2', summonerName: 'Blue 2', perks: {}, bot: false },
        { teamId: 100, championId: 3, spell1Id: 4, spell2Id: 12, puuid: 'blue-3', summonerName: 'Blue 3', perks: {}, bot: false },
        { teamId: 100, championId: 4, spell1Id: 4, spell2Id: 7, puuid: 'blue-4', summonerName: 'Blue 4', perks: {}, bot: false },
        { teamId: 100, championId: 5, spell1Id: 4, spell2Id: 14, puuid: 'blue-5', summonerName: 'Blue 5', perks: {}, bot: false },
        { teamId: 200, championId: 11, spell1Id: 4, spell2Id: 12, puuid: 'red-1', summonerName: 'Red 1', perks: {}, bot: false },
        { teamId: 200, championId: 12, spell1Id: 11, spell2Id: 4, puuid: 'red-2', summonerName: 'Red 2', perks: {}, bot: false },
        { teamId: 200, championId: 13, spell1Id: 4, spell2Id: 12, puuid: 'red-3', summonerName: 'Red 3', perks: {}, bot: false },
        { teamId: 200, championId: 14, spell1Id: 4, spell2Id: 7, puuid: 'red-4', summonerName: 'Red 4', perks: {}, bot: false },
        { teamId: 200, championId: 15, spell1Id: 4, spell2Id: 14, puuid: 'red-5', summonerName: 'Red 5', perks: {}, bot: false },
      ],
    });
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

    fireEvent.change(screen.getByPlaceholderText('Champion or Riot ID, e.g. Wukong or TWITCH ELOSANTA#1111'), {
      target: { value: 'Blue 1#NA1' },
    });
    fireEvent.click(screen.getByLabelText('Search WinRift'));

    expect(await screen.findByRole('button', { name: 'Show Win Conditions mode' })).toBeInTheDocument();
    expect(getWinConditionAnalysis).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Show Win Conditions mode' }));
    await waitFor(() => expect(getWinConditionAnalysis).toHaveBeenCalled());
    expect(await screen.findByText("Your Team's Win Condition")).toBeInTheDocument();
    expect(screen.getByText('Other Strategies')).toBeInTheDocument();
    expect(screen.getByText('Enemy Strategies')).toBeInTheDocument();
    expect(screen.getByText('Match Read')).toBeInTheDocument();
    expect(screen.getByText('Winrate By Game Length')).toBeInTheDocument();
    expect(screen.getByText("Enemy Team's Win Condition")).toBeInTheDocument();
    expect(screen.getByText('Your Pick B+')).toBeInTheDocument();
    expect(screen.getByText('Enemy Control B+')).toBeInTheDocument();
    expect(screen.getByText('Primary pick read')).toBeInTheDocument();
    expect(screen.getByLabelText('Win Rate: 55.00%')).toBeInTheDocument();
    expect(screen.getByLabelText('Total Games: 20')).toBeInTheDocument();
    const enemySplitPush = screen.getByLabelText('Show Enemy Strategies SplitPush');
    expect(within(enemySplitPush).getByText('SplitPush C-')).toBeInTheDocument();
    expect(enemySplitPush).not.toHaveTextContent('80%');
    expect(enemySplitPush).not.toHaveTextContent('20g');
    expect(screen.queryByLabelText('Show Enemy Strategies Control')).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText('Show Enemy Strategies Siege'));
    expect(await screen.findByLabelText('Win Rate: 70.00%')).toBeInTheDocument();
    expect(screen.getByText('Enemy Siege B')).toBeInTheDocument();
    expect(screen.getByLabelText('Show Enemy Strategies Control')).toBeInTheDocument();
    expect(screen.queryByLabelText('Show Enemy Strategies Siege')).not.toBeInTheDocument();
    expect(screen.getByText('Siege response read')).toBeInTheDocument();
    queryClient.clear();
  });
});

const winConditionFixture = vi.hoisted(() => {
  const scriptFixture = (id: string, playerRead: string) => ({
    id,
    headline: id,
    overview: `${id} overview`,
    matchup: `${id} matchup`,
    ratingRead: `${id} rating`,
    modeRead: `${id} mode`,
    timingRead: `${id} timing`,
    sampleRead: `${id} sample`,
    playerRead,
    facts: [],
  });
  const evidenceFixture = (level = 'Early', direction = 'mixed') => ({
    score: 42,
    level,
    direction,
    summary: `${level} ${direction} evidence`,
    wilsonLow: 30,
    wilsonHigh: 70,
    sampleLevel: 'early',
  });

  return {
  catalogPatch: '16.10.1',
  filters: {
    queueId: 420,
    patch: '',
    rankBucket: '',
    metricSource: 'raw',
    compiledMetricRows: 0,
    rawTeamRows: 20,
    filteredTeamRows: 20,
  },
  blue: {
    championIds: [1, 2, 3, 4, 5],
    scores: { splitpush: 8, pick: 16, siege: 8, control: 10, teamfight: 8 },
    ratings: { splitpush: 'C', pick: 'B+', siege: 'C', control: 'C+', teamfight: 'C' },
    axes: [
      { key: 'splitpush', label: 'SplitPush', score: 8, rating: 'C', deltaFromPrimary: 8, planRole: 'weak-angle', planLabel: 'Weak angle' },
      { key: 'pick', label: 'Pick', score: 16, rating: 'B+', deltaFromPrimary: 0, planRole: 'primary', planLabel: 'Primary' },
      { key: 'siege', label: 'Siege', score: 8, rating: 'C', deltaFromPrimary: 8, planRole: 'weak-angle', planLabel: 'Weak angle' },
      { key: 'control', label: 'Control', score: 10, rating: 'C+', deltaFromPrimary: 6, planRole: 'weak-angle', planLabel: 'Weak angle' },
      { key: 'teamfight', label: 'TeamFight', score: 8, rating: 'C', deltaFromPrimary: 8, planRole: 'weak-angle', planLabel: 'Weak angle' },
    ],
    primaryCondition: 'Pick',
    primaryScore: 16,
    primaryRating: 'B+',
    missingChampionIds: [],
  },
  red: {
    championIds: [11, 12, 13, 14, 15],
    scores: { splitpush: 6, pick: 8, siege: 14, control: 16, teamfight: 12 },
    ratings: { splitpush: 'C-', pick: 'C', siege: 'B', control: 'B+', teamfight: 'B-' },
    axes: [
      { key: 'splitpush', label: 'SplitPush', score: 6, rating: 'C-', deltaFromPrimary: 10, planRole: 'weak-angle', planLabel: 'Weak angle' },
      { key: 'pick', label: 'Pick', score: 8, rating: 'C', deltaFromPrimary: 8, planRole: 'weak-angle', planLabel: 'Weak angle' },
      { key: 'siege', label: 'Siege', score: 14, rating: 'B', deltaFromPrimary: 2, planRole: 'strong-secondary', planLabel: 'Strong secondary' },
      { key: 'control', label: 'Control', score: 16, rating: 'B+', deltaFromPrimary: 0, planRole: 'primary', planLabel: 'Primary' },
      { key: 'teamfight', label: 'TeamFight', score: 12, rating: 'B-', deltaFromPrimary: 4, planRole: 'secondary', planLabel: 'Secondary' },
    ],
    primaryCondition: 'Control',
    primaryScore: 16,
    primaryRating: 'B+',
    missingChampionIds: [],
  },
  blueMatchups: [
    {
      condition: 'SplitPush',
      rating: 'C',
      planRole: 'weak-angle',
      planLabel: 'Weak angle',
      deltaFromPrimary: 8,
      opponentCondition: 'Control',
      opponentRating: 'B+',
      opponentPlanRole: 'primary',
      opponentPlanLabel: 'Primary',
      primary: false,
      opponentPrimary: true,
      wins: 4,
      games: 8,
      winRate: 50,
      confidence: 22,
      evidence: evidenceFixture(),
      meetsMinGames: true,
      buckets: [],
    },
    {
      condition: 'Pick',
      rating: 'B+',
      planRole: 'primary',
      planLabel: 'Primary',
      deltaFromPrimary: 0,
      opponentCondition: 'Control',
      opponentRating: 'B+',
      opponentPlanRole: 'primary',
      opponentPlanLabel: 'Primary',
      primary: true,
      opponentPrimary: true,
      wins: 11,
      games: 20,
      winRate: 55,
      confidence: 34,
      evidence: evidenceFixture('Moderate', 'favorable'),
      meetsMinGames: true,
      buckets: [
        { bucket: '0-20', wins: 2, games: 4, winRate: 50, confidence: 15, meetsMinGames: false },
        { bucket: '20-25', wins: 3, games: 5, winRate: 60, confidence: 23, meetsMinGames: true },
        { bucket: '25-30', wins: 3, games: 5, winRate: 60, confidence: 23, meetsMinGames: true },
        { bucket: '30-35', wins: 2, games: 3, winRate: 66.7, confidence: 21, meetsMinGames: false },
        { bucket: '35+', wins: 1, games: 3, winRate: 33.3, confidence: 6, meetsMinGames: false },
      ],
      script: scriptFixture('pick-control', 'Primary pick read'),
    },
    {
      condition: 'Pick',
      rating: 'B+',
      planRole: 'primary',
      planLabel: 'Primary',
      deltaFromPrimary: 0,
      opponentCondition: 'Siege',
      opponentRating: 'B',
      opponentPlanRole: 'strong-secondary',
      opponentPlanLabel: 'Strong secondary',
      primary: true,
      opponentPrimary: false,
      wins: 14,
      games: 20,
      winRate: 70,
      confidence: 49,
      evidence: evidenceFixture('Strong', 'favorable'),
      meetsMinGames: true,
      buckets: [
        { bucket: '0-20', wins: 4, games: 5, winRate: 80, confidence: 38, meetsMinGames: true },
        { bucket: '20-25', wins: 3, games: 5, winRate: 60, confidence: 23, meetsMinGames: true },
        { bucket: '25-30', wins: 3, games: 4, winRate: 75, confidence: 30, meetsMinGames: false },
        { bucket: '30-35', wins: 2, games: 3, winRate: 66.7, confidence: 21, meetsMinGames: false },
        { bucket: '35+', wins: 2, games: 3, winRate: 66.7, confidence: 21, meetsMinGames: false },
      ],
      script: scriptFixture('pick-siege', 'Siege response read'),
    },
  ],
  redMatchups: [
    {
      condition: 'Control',
      rating: 'B+',
      planRole: 'primary',
      planLabel: 'Primary',
      deltaFromPrimary: 0,
      opponentCondition: 'Pick',
      opponentRating: 'B+',
      opponentPlanRole: 'primary',
      opponentPlanLabel: 'Primary',
      primary: true,
      opponentPrimary: true,
      wins: 9,
      games: 20,
      winRate: 45,
      confidence: 26,
      evidence: evidenceFixture('Moderate', 'unfavorable'),
      meetsMinGames: true,
      buckets: [],
    },
    {
      condition: 'Siege',
      rating: 'B',
      planRole: 'strong-secondary',
      planLabel: 'Strong secondary',
      deltaFromPrimary: 2,
      opponentCondition: 'Pick',
      opponentRating: 'B+',
      opponentPlanRole: 'primary',
      opponentPlanLabel: 'Primary',
      primary: false,
      opponentPrimary: true,
      wins: 6,
      games: 20,
      winRate: 30,
      confidence: 15,
      evidence: evidenceFixture('Early', 'unfavorable'),
      meetsMinGames: true,
      buckets: [],
    },
    {
      condition: 'SplitPush',
      rating: 'C-',
      planRole: 'weak-angle',
      planLabel: 'Weak angle',
      deltaFromPrimary: 10,
      opponentCondition: 'Pick',
      opponentRating: 'B+',
      opponentPlanRole: 'primary',
      opponentPlanLabel: 'Primary',
      primary: false,
      opponentPrimary: true,
      wins: 16,
      games: 20,
      winRate: 80,
      confidence: 52,
      evidence: evidenceFixture('Strong', 'favorable'),
      meetsMinGames: true,
      buckets: [],
    },
  ],
  };
});
