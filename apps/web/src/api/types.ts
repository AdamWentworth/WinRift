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
  title?: string;
  image: {
    full: string;
  };
  passive?: ChampionAbility;
  spells?: ChampionAbility[];
};

export type ChampionAbility = {
  id: string;
  name: string;
  description?: string;
  image?: {
    full: string;
  };
};

export type ChampionSplashData = {
  version: string;
  data: ChampionSplash[];
};

export type ChampionSplash = {
  championId: string;
  championName: string;
  skinName: string;
  skinNumber: number;
  src: string;
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
  sampleScope?: string;
  sampleScopeLabel?: string;
  fallback?: boolean;
};

export type AnalyticsItemSlotResponse = {
  results: AnalyticsItemSlot[];
};

export type AnalyticsItemSlotBatchRequest = BuildFilters & {
  key: string;
};

export type AnalyticsItemSlotBatchResult = {
  key: string;
  results: AnalyticsItemSlot[];
};

export type AnalyticsItemSlotBatchResponse = {
  results: AnalyticsItemSlotBatchResult[];
};

export type ChampionGuideSummary = {
  championId: number;
  role: string;
  patchBucket: string;
  rankBucket: string;
  wins: number;
  games: number;
  bans: number;
  winRate: number;
  confidence: number;
  pickRate: number;
  banRate: number;
  avgKills?: number;
  avgDeaths?: number;
  avgAssists?: number;
  kda?: number;
  avgGoldEarned?: number;
  avgCs?: number;
  avgDamageDealtToChampions?: number;
  avgDamageTaken?: number;
  avgDamageSelfMitigated?: number;
  avgDamageDealtToObjectives?: number;
  avgDamageDealtToStructures?: number;
  avgVisionScore?: number;
  avgTimeCcingOthers?: number;
  avgTeamUtility?: number;
  avgStructureTakedowns?: number;
  avgObjectiveTakedowns?: number;
  avgTotalTimeSpentDead?: number;
  avgTimePlayed?: number;
  killParticipation?: number;
  tierScore?: number;
  winScore?: number;
  sampleScore?: number;
  pickScore?: number;
  banScore?: number;
  impactScore?: number;
  damageScore?: number;
  economyScore?: number;
  visionScore?: number;
  objectiveScore?: number;
  utilityScore?: number;
  survivabilityScore?: number;
  roleRank: number;
  roleRankTotal: number;
};

export type ChampionGuideMatchup = {
  opponentChampionId: number;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type ChampionGuideRune = {
  runeSignature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type ChampionGuideSpells = {
  spellSignature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type ChampionGuideSkillOrder = {
  skillOrderSignature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type ChampionGuideItemPath = {
  core3Signature: string;
  finalItemsSignature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
};

export type ChampionGuideResponse = {
  summary: ChampionGuideSummary;
  toughestMatchups: ChampionGuideMatchup[];
  bestMatchups: ChampionGuideMatchup[];
  topRunes: ChampionGuideRune[];
  topSpells: ChampionGuideSpells[];
  topSkillOrders: ChampionGuideSkillOrder[];
  topItemPaths: ChampionGuideItemPath[];
};

export type ChampionGuideIndexResponse = {
  results: ChampionGuideSummary[];
};

export type ChampionRoleRate = {
  championId: number;
  role: string;
  games: number;
  totalGames: number;
  pickRate: number;
};

export type ChampionRoleRatesResponse = {
  results: ChampionRoleRate[];
};

export type WinConditionAnalysisRequest = {
  blueChampionIds: number[];
  redChampionIds: number[];
  queueId: number;
  patch?: string;
  rankBucket?: string;
  minGames?: number;
  maxRows?: number;
};

export type WinConditionTeamProfile = {
  championIds: number[];
  scores: {
    splitpush: number;
    pick: number;
    siege: number;
    control: number;
    teamfight: number;
  };
  ratings: Record<string, string>;
  axes: WinConditionAxisScore[];
  primaryCondition: string;
  primaryScore: number;
  primaryRating: string;
  primaryMargin?: number;
  sharpness?: string;
  sharpnessLabel?: string;
  missingChampionIds: number[];
};

export type WinConditionAxisScore = {
  key: string;
  label: string;
  score: number;
  rating: string;
  deltaFromPrimary?: number;
  planRole?: string;
  planLabel?: string;
};

export type WinConditionBucket = {
  bucket: string;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
  meetsMinGames: boolean;
};

export type WinConditionMetric = {
  condition: string;
  rating: string;
  planRole?: string;
  planLabel?: string;
  deltaFromPrimary?: number;
  opponentCondition: string;
  opponentRating: string;
  opponentPlanRole?: string;
  opponentPlanLabel?: string;
  primary: boolean;
  opponentPrimary: boolean;
  wins: number;
  games: number;
  winRate: number;
  confidence: number;
  evidence: WinConditionEvidence;
  meetsMinGames: boolean;
  buckets: WinConditionBucket[];
  script?: WinConditionScript;
};

export type WinConditionEvidence = {
  score: number;
  level: string;
  direction: string;
  summary: string;
  wilsonLow: number;
  wilsonHigh: number;
  sampleLevel: string;
};

export type WinConditionScript = {
  id: string;
  headline: string;
  overview: string;
  matchup: string;
  ratingRead: string;
  modeRead: string;
  timingRead: string;
  cautionRead?: string;
  sampleRead: string;
  playerRead: string;
  facts: string[];
};

export type WinConditionAnalysisResponse = {
  catalogPatch: string;
  filters: {
    queueId: number;
    patch: string;
    rankBucket: string;
    metricSource: string;
    compiledMetricRows: number;
    rawTeamRows: number;
    filteredTeamRows: number;
  };
  blue: WinConditionTeamProfile;
  red: WinConditionTeamProfile;
  blueMatchups: WinConditionMetric[];
  redMatchups: WinConditionMetric[];
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
  itemContext?: 'JUNGLE' | 'SUPPORT';
  opponentChampionId?: number;
  patch?: string;
  rankBucket?: string;
  minGames: number;
  limit?: number;
  fallback?: boolean;
};
