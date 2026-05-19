export type ChampionData = {
  version: string;
  data: {
    data: Record<string, Champion>;
  };
};

export type Champion = {
  id: string;
  key: string;
  name: string;
  image: {
    full: string;
  };
};

export type ItemData = {
  version: string;
  data: {
    data: Record<string, Item>;
  };
};

export type Item = {
  name: string;
  plaintext?: string;
  image?: {
    full: string;
  };
};

export type SummonerSpellData = {
  version: string;
  data: {
    data: Record<string, SummonerSpell>;
  };
};

export type SummonerSpell = {
  key: string;
  name: string;
  image?: {
    full: string;
  };
};

export type RuneData = {
  version: string;
  data: RuneStyle[];
};

export type RuneStyle = {
  id: number;
  key: string;
  name: string;
  icon: string;
  slots: {
    runes: Rune[];
  }[];
};

export type Rune = {
  id: number;
  key: string;
  name: string;
  icon: string;
};

export type AnalyticsBuild = {
  championId: number;
  role: string;
  opponentChampionId: number;
  patchBucket: string;
  rankBucket: string;
  finalItemsSignature: string;
  core2Signature: string;
  core3Signature: string;
  runeSignature: string;
  spellSignature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type AnalyticsBuildResponse = {
  results: AnalyticsBuild[];
};

export type AnalyticsItemSlot = {
  championId: number;
  role: string;
  opponentChampionId: number;
  patchBucket: string;
  rankBucket: string;
  itemSlot: number;
  itemId: number;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type AnalyticsItemSlotResponse = {
  results: AnalyticsItemSlot[];
};

export type AccountAliasResolution = {
  status: 'found' | 'ambiguous' | 'not_found';
  puuid?: string;
  platform?: string;
  gameName?: string;
  tagLine?: string;
  matches?: AccountAliasMatch[];
};

export type AccountAliasSearchResponse = {
  matches: AccountAliasMatch[];
};

export type AccountAliasMatch = {
  puuid: string;
  platform: string;
  gameName: string;
  tagLine: string;
};

export type RankedRecord = {
  queueType: string;
  tier: string;
  division: string;
  rank?: string;
  leaguePoints: number;
  wins: number;
  losses: number;
  totalGames: number;
  winRate: number;
  rankBucket: string;
  fetchedAt?: string;
  expiresAt?: string;
  rankAvailable?: boolean;
};

export type ChampionRecord = {
  queueId: number;
  championId: number;
  games: number;
  wins: number;
  losses: number;
  kills: number;
  deaths: number;
  assists: number;
  avgKills: number;
  avgDeaths: number;
  avgAssists: number;
  kda: number;
  winRate: number;
};

export type LiveParticipant = {
  teamId: number;
  championId: number;
  spell1Id: number;
  spell2Id: number;
  puuid?: string;
  summonerId?: string;
  riotId?: string;
  summonerName?: string;
  rank?: RankedRecord;
  championStats?: ChampionRecord;
  perks: {
    perkIds?: number[];
    perkStyle?: number;
    perkSubStyle?: number;
    statPerks?: {
      offense?: number;
      flex?: number;
      defense?: number;
    };
    styles?: {
      description?: string;
      style?: number;
      selections?: {
        perk: number;
      }[];
    }[];
  };
  bot: boolean;
};

export type LiveGame = {
  platform: string;
  puuid: string;
  gameId: number;
  mapId: number;
  gameMode: string;
  gameType: string;
  gameQueueConfigId: number;
  gameStartTime: number;
  participants: LiveParticipant[];
};

export type BuildFilters = {
  championId?: number;
  role?: string;
  opponentChampionId?: number;
  patch?: string;
  rankBucket?: string;
  minGames: number;
  limit?: number;
};
