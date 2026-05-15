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

export type LiveParticipant = {
  teamId: number;
  championId: number;
  spell1Id: number;
  spell2Id: number;
  summonerId?: string;
  riotId: string;
  perks: {
    perkIds?: number[];
    perkStyle?: number;
    perkSubStyle?: number;
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
};
