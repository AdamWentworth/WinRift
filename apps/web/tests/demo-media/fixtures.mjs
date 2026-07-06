const version = '16.10.1';
const patch = '16.10';

const championDefs = [
  ['Aatrox', '266', 'Aatrox', 'the Darkin Blade'],
  ['Ahri', '103', 'Ahri', 'the Nine-Tailed Fox'],
  ['Akali', '84', 'Akali', 'the Rogue Assassin'],
  ['Annie', '1', 'Annie', 'the Dark Child'],
  ['Ashe', '22', 'Ashe', 'the Frost Archer'],
  ['Caitlyn', '51', 'Caitlyn', 'the Sheriff of Piltover'],
  ['Galio', '3', 'Galio', 'the Colossus'],
  ['Jinx', '222', 'Jinx', 'the Loose Cannon'],
  ['Kaisa', '145', "Kai'Sa", 'Daughter of the Void'],
  ['LeeSin', '64', 'Lee Sin', 'the Blind Monk'],
  ['Leona', '89', 'Leona', 'the Radiant Dawn'],
  ['Lux', '99', 'Lux', 'the Lady of Luminosity'],
  ['MonkeyKing', '62', 'Wukong', 'the Monkey King'],
  ['Nami', '267', 'Nami', 'the Tidecaller'],
  ['Olaf', '2', 'Olaf', 'the Berserker'],
  ['Orianna', '61', 'Orianna', 'the Lady of Clockwork'],
  ['Riven', '92', 'Riven', 'the Exile'],
  ['Sion', '14', 'Sion', 'the Undead Juggernaut'],
  ['Sivir', '15', 'Sivir', 'the Battle Mistress'],
  ['Thresh', '412', 'Thresh', 'the Chain Warden'],
  ['TwistedFate', '4', 'Twisted Fate', 'the Card Master'],
  ['Vayne', '67', 'Vayne', 'the Night Hunter'],
  ['XinZhao', '5', 'Xin Zhao', 'the Seneschal of Demacia'],
  ['Yasuo', '157', 'Yasuo', 'the Unforgiven'],
];

export const championsResponse = {
  version,
  data: {
    data: Object.fromEntries(championDefs.map(([id, key, name, title]) => [
      id,
      {
        id,
        key,
        name,
        title,
        image: { full: `${id}.png` },
        passive: { id: `${id}Passive`, name: `${name} Passive` },
        spells: ['Q', 'W', 'E', 'R'].map((slot) => ({
          id: `${id}${slot}`,
          name: `${name} ${slot}`,
        })),
      },
    ])),
  },
};

export const championSplashesResponse = {
  version,
  data: championDefs.map(([id, key, name]) => ({
    championId: id,
    championName: name,
    skinName: 'Classic',
    skinNumber: 0,
    src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${id}_0.jpg`,
    key,
  })),
};

export const itemsResponse = {
  version,
  data: {
    data: Object.fromEntries([
      ['1054', "Doran's Shield"],
      ['1055', "Doran's Blade"],
      ['2003', 'Health Potion'],
      ['3006', "Berserker's Greaves"],
      ['3031', 'Infinity Edge'],
      ['3036', "Lord Dominik's Regards"],
      ['3047', 'Plated Steelcaps'],
      ['3065', 'Spirit Visage'],
      ['3071', 'Black Cleaver'],
      ['3072', 'Bloodthirster'],
      ['3075', 'Thornmail'],
      ['3078', 'Trinity Force'],
      ['3094', 'Rapid Firecannon'],
      ['3111', "Mercury's Treads"],
      ['3158', 'Ionian Boots of Lucidity'],
      ['6333', "Death's Dance"],
      ['6672', 'Kraken Slayer'],
      ['6676', 'The Collector'],
      ['6692', 'Eclipse'],
    ].map(([id, name]) => [id, { name, plaintext: '', tags: [], image: { full: `${id}.png` } }])),
  },
};

export const spellsResponse = {
  version,
  data: {
    data: {
      SummonerFlash: { key: '4', name: 'Flash', image: { full: 'SummonerFlash.png' } },
      SummonerTeleport: { key: '12', name: 'Teleport', image: { full: 'SummonerTeleport.png' } },
      SummonerSmite: { key: '11', name: 'Smite', image: { full: 'SummonerSmite.png' } },
      SummonerDot: { key: '14', name: 'Ignite', image: { full: 'SummonerDot.png' } },
      SummonerHeal: { key: '7', name: 'Heal', image: { full: 'SummonerHeal.png' } },
      SummonerExhaust: { key: '3', name: 'Exhaust', image: { full: 'SummonerExhaust.png' } },
    },
  },
};

export const runesResponse = {
  version,
  data: [
    runeStyle(8000, 'Precision', 'perk-images/Styles/7201_Precision.png', [
      rune(8005, 'Press the Attack', 'perk-images/Styles/Precision/PressTheAttack/PressTheAttack.png'),
      rune(8010, 'Conqueror', 'perk-images/Styles/Precision/Conqueror/Conqueror.png'),
      rune(9101, 'Absorb Life', 'perk-images/Styles/Precision/AbsorbLife/AbsorbLife.png'),
      rune(9104, 'Legend: Alacrity', 'perk-images/Styles/Precision/LegendAlacrity/LegendAlacrity.png'),
      rune(8014, 'Coup de Grace', 'perk-images/Styles/Precision/CoupDeGrace/CoupDeGrace.png'),
    ]),
    runeStyle(8100, 'Domination', 'perk-images/Styles/7200_Domination.png', [
      rune(8112, 'Electrocute', 'perk-images/Styles/Domination/Electrocute/Electrocute.png'),
      rune(8128, 'Dark Harvest', 'perk-images/Styles/Domination/DarkHarvest/DarkHarvest.png'),
      rune(8143, 'Sudden Impact', 'perk-images/Styles/Domination/SuddenImpact/SuddenImpact.png'),
      rune(8138, 'Eyeball Collection', 'perk-images/Styles/Domination/EyeballCollection/EyeballCollection.png'),
      rune(8135, 'Treasure Hunter', 'perk-images/Styles/Domination/TreasureHunter/TreasureHunter.png'),
    ]),
    runeStyle(8200, 'Sorcery', 'perk-images/Styles/7202_Sorcery.png', [
      rune(8214, 'Summon Aery', 'perk-images/Styles/Sorcery/SummonAery/SummonAery.png'),
      rune(8226, 'Manaflow Band', 'perk-images/Styles/Sorcery/ManaflowBand/ManaflowBand.png'),
      rune(8233, 'Absolute Focus', 'perk-images/Styles/Sorcery/AbsoluteFocus/AbsoluteFocus.png'),
      rune(8236, 'Gathering Storm', 'perk-images/Styles/Sorcery/GatheringStorm/GatheringStorm.png'),
    ]),
    runeStyle(8400, 'Resolve', 'perk-images/Styles/7204_Resolve.png', [
      rune(8437, 'Grasp of the Undying', 'perk-images/Styles/Resolve/GraspOfTheUndying/GraspOfTheUndying.png'),
      rune(8446, 'Demolish', 'perk-images/Styles/Resolve/Demolish/Demolish.png'),
      rune(8444, 'Second Wind', 'perk-images/Styles/Resolve/SecondWind/SecondWind.png'),
      rune(8451, 'Overgrowth', 'perk-images/Styles/Resolve/Overgrowth/Overgrowth.png'),
    ]),
  ],
};

const guideSummaries = [
  championSummary(266, 'TOP', 1, 4320, 53.6, 9.8, 11.3, 87.8, 62.4),
  championSummary(64, 'JUNGLE', 1, 3890, 52.9, 8.7, 14.1, 85.1, 61.8),
  championSummary(103, 'MIDDLE', 2, 3615, 52.4, 7.2, 8.4, 82.7, 60.5),
  championSummary(145, 'BOTTOM', 1, 5060, 53.1, 13.2, 18.4, 88.3, 63.9),
  championSummary(412, 'UTILITY', 1, 3180, 51.9, 6.6, 9.1, 79.3, 58.2),
  championSummary(157, 'MIDDLE', 5, 2950, 50.8, 8.1, 21.2, 76.4, 54.8),
  championSummary(222, 'BOTTOM', 3, 4200, 51.7, 10.5, 7.3, 77.1, 56.9),
  championSummary(99, 'MIDDLE', 8, 2450, 50.6, 5.1, 4.2, 72.3, 51.4),
  championSummary(62, 'JUNGLE', 4, 2200, 51.1, 4.4, 3.2, 70.8, 50.7),
  championSummary(22, 'BOTTOM', 7, 2055, 50.2, 4.8, 2.5, 68.6, 49.3),
  championSummary(89, 'UTILITY', 3, 1984, 51.0, 3.9, 5.6, 71.2, 50.8),
  championSummary(92, 'TOP', 6, 1842, 49.8, 4.6, 6.5, 69.1, 48.6),
];

export function responseForApiRequest(requestUrl, method, body) {
  const url = new URL(requestUrl, 'http://winrift-demo.local');
  const path = url.pathname;

  if (path === '/api/static/champions') return ok(championsResponse);
  if (path === '/api/static/champion-splashes') return ok(championSplashesResponse);
  if (path === '/api/static/items') return ok(itemsResponse);
  if (path === '/api/static/summoner-spells') return ok(spellsResponse);
  if (path === '/api/static/runes') return ok(runesResponse);
  if (path === '/api/analytics/patches') return ok(analyticsPatchesResponse());
  if (path === '/api/analytics/champion-guides') return ok(championGuideIndexResponse(url));
  if (path === '/api/analytics/champion-page') return ok(championPageBundleResponse(url));
  if (path === '/api/analytics/champion-roles') return ok(championRoleRatesResponse(url));
  if (path === '/api/analytics/build-advice') return ok(buildAdviceResponse(url));
  if (path === '/api/analytics/win-conditions' && method === 'POST') return ok(winConditionResponse(body));
  if (path === '/api/live-game') return liveGameResponse(url);
  if (path === '/api/summoner/profile') return ok(summonerProfileResponse(url));
  if (path === '/api/summoners/leaderboard') return ok(summonerLeaderboardResponse(url));
  if (path === '/api/account/alias') return ok(accountAliasResponse(url));
  if (path === '/api/account/aliases') return ok(accountAliasSearchResponse(url));

  return { status: 404, body: { detail: `No demo fixture for ${method} ${path}` } };
}

function ok(body) {
  return { status: 200, body };
}

function rune(id, name, icon) {
  return { id, key: name.replace(/[^A-Za-z0-9]/g, ''), name, icon };
}

function runeStyle(id, name, icon, runes) {
  return {
    id,
    key: name.replace(/[^A-Za-z0-9]/g, ''),
    name,
    icon,
    slots: [
      { runes: runes.slice(0, 2) },
      { runes: runes.slice(2, 3) },
      { runes: runes.slice(3, 4) },
      { runes: runes.slice(4, 5) },
    ],
  };
}

function championSummary(championId, role, roleRank, games, winRate, pickRate, banRate, tierScore, impactScore) {
  const wins = Math.round(games * (winRate / 100));
  return {
    championId,
    role,
    patchBucket: patch,
    rankBucket: 'ALL',
    wins,
    games,
    bans: Math.round(games * (banRate / 100)),
    winRate,
    confidence: Math.min(99, tierScore - 10),
    pickRate,
    banRate,
    avgKills: 7.2,
    avgDeaths: 4.1,
    avgAssists: 8.4,
    kda: 3.8,
    avgGoldEarned: 12240,
    avgCs: 192,
    avgDamageDealtToChampions: 24100,
    avgDamageTaken: 26500,
    avgDamageSelfMitigated: 18400,
    avgDamageDealtToObjectives: 7200,
    avgDamageDealtToStructures: 3800,
    avgVisionScore: 28,
    avgTimeCcingOthers: 32,
    avgTeamUtility: 48,
    avgStructureTakedowns: 2.1,
    avgObjectiveTakedowns: 1.6,
    avgTotalTimeSpentDead: 210,
    avgTimePlayed: 1860,
    killParticipation: 61.4,
    tierScore,
    winScore: winRate,
    sampleScore: Math.min(100, games / 55),
    pickScore: pickRate,
    banScore: banRate,
    impactScore,
    damageScore: impactScore - 8,
    economyScore: impactScore - 12,
    visionScore: role === 'UTILITY' ? impactScore + 8 : impactScore - 20,
    objectiveScore: role === 'JUNGLE' ? impactScore + 7 : impactScore - 10,
    utilityScore: role === 'UTILITY' ? impactScore + 6 : impactScore - 16,
    survivabilityScore: impactScore - 4,
    roleRank,
    roleRankTotal: 48,
  };
}

function analyticsPatchesResponse() {
  return {
    currentPatch: patch,
    queueId: 420,
    results: [
      { patch, matches: 112480, participantSamples: 1124800, rawMatches: 118200, compiledMatches: 112480, current: true },
      { patch: '16.9', matches: 98430, participantSamples: 984300, rawMatches: 100180, compiledMatches: 98430, current: false },
    ],
  };
}

function championGuideIndexResponse(url) {
  const role = url.searchParams.get('role') ?? '';
  const filtered = role ? guideSummaries.filter((row) => row.role === role) : guideSummaries;
  return {
    results: filtered,
    matchCount: 112480,
    participantSamples: filtered.reduce((sum, row) => sum + row.games, 0),
  };
}

function championPageBundleResponse(url) {
  const championId = Number(url.searchParams.get('championId') ?? 266);
  const summary = summaryForChampion(championId);
  const role = url.searchParams.get('role') || summary.role;
  const guide = championGuide(championId, role);
  return {
    filters: {
      championId,
      role,
      opponentChampionId: Number(url.searchParams.get('opponentChampionId') ?? 0),
      patch,
      rankBucket: url.searchParams.get('rankBucket') ?? '',
      queueId: 420,
    },
    guide,
    buildAdvice: buildAdviceFor(championId, role, Number(url.searchParams.get('opponentChampionId') ?? 0)),
    guideIndex: championGuideIndexResponse(url),
    roleRates: championRoleRatesResponse(url),
  };
}

function championGuide(championId, role) {
  const summary = { ...summaryForChampion(championId), role };
  return {
    summary,
    toughestMatchups: [
      { opponentChampionId: 14, wins: 82, games: 180, winRate: 45.6, confidence: 47 },
      { opponentChampionId: 89, wins: 96, games: 203, winRate: 47.3, confidence: 51 },
      { opponentChampionId: 99, wins: 72, games: 149, winRate: 48.3, confidence: 43 },
    ],
    bestMatchups: [
      { opponentChampionId: 2, wins: 132, games: 220, winRate: 60.0, confidence: 58 },
      { opponentChampionId: 4, wins: 118, games: 201, winRate: 58.7, confidence: 55 },
      { opponentChampionId: 15, wins: 104, games: 182, winRate: 57.1, confidence: 52 },
    ],
    topRunes: [
      { runeSignature: '8000|8100|8010-9101-9104-8014|5008-5008-5011', wins: 904, games: 1640, winRate: 55.1, confidence: 72 },
      { runeSignature: '8100|8000|8112-8143-8138-8135|5008-5008-5011', wins: 532, games: 1015, winRate: 52.4, confidence: 64 },
    ],
    topSpells: [
      { spellSignature: role === 'JUNGLE' ? '4-11' : '4-12', wins: 1190, games: 2240, winRate: 53.1, confidence: 72 },
      { spellSignature: role === 'UTILITY' ? '4-3' : '4-14', wins: 722, games: 1398, winRate: 51.6, confidence: 61 },
    ],
    topSkillOrders: [
      { skillOrderSignature: '1-3-2-1-1-4-1-3-1-3-4-3-3-2-2-4-2-2', wins: 620, games: 1160, winRate: 53.4, confidence: 68 },
    ],
    topItemPaths: [
      { core3Signature: '6672-3006-3031', finalItemsSignature: '6672-3006-3031-3094-3036-3072', wins: 620, games: 1120, winRate: 55.4, confidence: 70 },
      { core3Signature: '6692-3071-3158', finalItemsSignature: '6692-3071-3158-6333-3065-3075', wins: 410, games: 790, winRate: 51.9, confidence: 58 },
    ],
    buildVariants: [
      {
        variantKey: `${championId}-ad`,
        variantLabel: 'AD pressure',
        variantTags: ['damage', 'snowball'],
        core2Signature: '6672-3031',
        core3Signature: '6672-3006-3031',
        finalItemsSignature: '6672-3006-3031-3094-3036-3072',
        runeSignature: '8000|8100|8010-9101-9104-8014|5008-5008-5011',
        spellSignature: role === 'JUNGLE' ? '4-11' : '4-12',
        skillOrderSignature: '1-3-2-1-1-4-1-3-1-3-4-3-3-2-2-4-2-2',
        skillOrderWins: 420,
        skillOrderGames: 810,
        skillOrderWinRate: 51.9,
        skillOrderConfidence: 66,
        wins: 620,
        games: 1120,
        winRate: 55.4,
        confidence: 70,
        buildCount: 16,
      },
      {
        variantKey: `${championId}-bruiser`,
        variantLabel: 'Bruiser pivot',
        variantTags: ['durable', 'frontline'],
        core2Signature: '6692-3071',
        core3Signature: '6692-3071-3158',
        finalItemsSignature: '6692-3071-3158-6333-3065-3075',
        runeSignature: '8000|8400|8010-9101-9104-8014|5008-5008-5011',
        spellSignature: role === 'JUNGLE' ? '4-11' : '4-14',
        skillOrderSignature: '1-3-2-1-1-4-1-3-1-3-4-3-3-2-2-4-2-2',
        skillOrderWins: 310,
        skillOrderGames: 590,
        skillOrderWinRate: 52.5,
        skillOrderConfidence: 59,
        wins: 410,
        games: 790,
        winRate: 51.9,
        confidence: 58,
        buildCount: 11,
      },
    ],
  };
}

function buildAdviceResponse(url) {
  const championId = Number(url.searchParams.get('championId') ?? 266);
  const role = url.searchParams.get('role') || summaryForChampion(championId).role;
  const opponentChampionId = Number(url.searchParams.get('opponentChampionId') ?? 0);
  return buildAdviceFor(championId, role, opponentChampionId);
}

function buildAdviceFor(championId, role, opponentChampionId) {
  const guide = championGuide(championId, role);
  const matchupSlots = itemSlots(championId, role, opponentChampionId || 14, false);
  const championSlots = itemSlots(championId, role, 0, true);
  return {
    filters: {
      championId,
      role,
      opponentChampionId,
      patch,
      rankBucket: '',
      itemContext: role === 'JUNGLE' ? 'JUNGLE' : role === 'UTILITY' ? 'SUPPORT' : '',
      minGames: 5,
      championMinGames: 10,
      limit: 12,
    },
    matchup: {
      available: true,
      itemSlots: matchupSlots,
      startingLoadouts: startingLoadouts(championId, role, opponentChampionId),
      topBuilds: guide.topItemPaths.map((row) => ({
        championId,
        role,
        opponentChampionId,
        patchBucket: patch,
        rankBucket: 'ALL',
        finalItemsSignature: row.finalItemsSignature,
        core2Signature: row.core3Signature.split('-').slice(0, 2).join('-'),
        core3Signature: row.core3Signature,
        runeSignature: guide.topRunes[0].runeSignature,
        spellSignature: guide.topSpells[0].spellSignature,
        wins: row.wins,
        games: row.games,
        winRate: row.winRate,
        confidence: row.confidence,
      })),
      sample: {
        maxGames: Math.max(...matchupSlots.map((row) => row.games)),
        optionCount: matchupSlots.length,
        fallbackUsed: false,
        scopeLabels: ['Current patch exact matchup'],
        sampleQuality: 'strong',
        sampleQualityLabel: 'Strong sample',
      },
      sampleMode: 'champion_matchup',
    },
    champion: {
      available: true,
      itemSlots: championSlots,
      startingLoadouts: startingLoadouts(championId, role, 0),
      topBuilds: guide.topItemPaths.map((row) => ({
        championId,
        role,
        opponentChampionId: 0,
        patchBucket: patch,
        rankBucket: 'ALL',
        finalItemsSignature: row.finalItemsSignature,
        core2Signature: row.core3Signature.split('-').slice(0, 2).join('-'),
        core3Signature: row.core3Signature,
        runeSignature: guide.topRunes[0].runeSignature,
        spellSignature: guide.topSpells[0].spellSignature,
        wins: row.wins,
        games: row.games,
        winRate: row.winRate,
        confidence: row.confidence,
      })),
      topRunes: guide.topRunes,
      topSpells: guide.topSpells,
      topItemPaths: guide.topItemPaths,
      buildVariants: guide.buildVariants,
      summary: guide.summary,
      sample: {
        maxGames: Math.max(...championSlots.map((row) => row.games)),
        optionCount: championSlots.length,
        fallbackUsed: false,
        scopeLabels: ['Current patch champion overall'],
        sampleQuality: 'strong',
        sampleQualityLabel: 'Strong sample',
      },
      sampleMode: 'champion_overall',
      strictRoleUsed: true,
    },
    diagnostics: {
      matchup: diagnosticsFor(matchupSlots),
      champion: diagnosticsFor(championSlots),
    },
    notes: ['Exact matchup data is available for this demo sample; champion-wide rows remain visible for comparison.'],
  };
}

function itemSlots(championId, role, opponentChampionId, championWide) {
  const main = role === 'JUNGLE'
    ? [6692, 3071, 3158, 6333, 3065, 3075]
    : role === 'UTILITY'
      ? [3047, 3075, 3065, 3071, 6333, 3078]
      : [6672, 3006, 3031, 3094, 3036, 3072];
  const alt = role === 'JUNGLE'
    ? [3078, 3047, 3071, 6333, 3065, 3075]
    : [6692, 3158, 3071, 6333, 3065, 3075];
  const rows = [
    slotRow(championId, role, opponentChampionId, 0, role === 'UTILITY' ? 1054 : 1055, 540, 53.1, championWide),
    slotRow(championId, role, opponentChampionId, 0, 2003, 430, 51.2, championWide),
  ];
  main.forEach((itemId, index) => {
    rows.push(slotRow(championId, role, opponentChampionId, index + 1, itemId, 820 - index * 84, 55.8 - index * 0.7, championWide));
  });
  alt.slice(0, 4).forEach((itemId, index) => {
    rows.push(slotRow(championId, role, opponentChampionId, index + 1, itemId, 320 - index * 42, 51.6 + index * 0.4, championWide));
  });
  return rows;
}

function slotRow(championId, role, opponentChampionId, itemSlot, itemId, games, winRate, championWide) {
  return {
    championId,
    role,
    opponentChampionId,
    patchBucket: patch,
    rankBucket: 'ALL',
    itemSlot,
    itemId,
    wins: Math.round(games * (winRate / 100)),
    games,
    winRate,
    confidence: Math.min(92, 42 + games / 18),
    sampleScope: championWide ? 'champion_overall' : 'champion_matchup',
    sampleScopeLabel: championWide ? 'Current patch champion overall' : 'Current patch exact matchup',
    fallback: false,
  };
}

function startingLoadouts(championId, role, opponentChampionId) {
  return [
    { championId, role, opponentChampionId, patchBucket: patch, rankBucket: 'ALL', itemSignature: role === 'UTILITY' ? '1054-2003-2003' : '1055-2003', wins: 620, games: 1120, winRate: 55.4, confidence: 70, sampleQuality: 'strong', sampleQualityLabel: 'Strong sample', confidencePercentage: 70 },
    { championId, role, opponentChampionId, patchBucket: patch, rankBucket: 'ALL', itemSignature: '1054-2003', wins: 340, games: 660, winRate: 51.5, confidence: 58, sampleQuality: 'moderate', sampleQualityLabel: 'Moderate sample', confidencePercentage: 58 },
  ];
}

function diagnosticsFor(rows) {
  return {
    selectedSlots: rows.slice(0, 6).map((row) => ({
      slot: row.itemSlot,
      missing: false,
      candidateCount: 2,
      itemId: row.itemId,
      games: row.games,
      winRate: row.winRate,
      sampleScope: row.sampleScope,
      sampleScopeLabel: row.sampleScopeLabel,
      fallback: false,
      opponentChampionId: row.opponentChampionId,
    })),
    missingSlots: [],
    fallbackSlots: [],
    currentPatchExactSlots: [1, 2, 3, 4, 5, 6],
    allPatchExactSlots: [],
    championWideSlots: [],
    scopeCounts: [{ scope: 'champion_matchup', label: 'Current patch exact matchup', fallback: false, rows: rows.length }],
  };
}

function championRoleRatesResponse(url) {
  const requested = (url.searchParams.get('championIds') ?? '')
    .split(',')
    .map(Number)
    .filter(Boolean);
  const ids = requested.length ? requested : guideSummaries.map((row) => row.championId);
  return {
    results: ids.map((championId) => {
      const summary = summaryForChampion(championId);
      return { championId, role: summary.role, games: summary.games, totalGames: summary.games + 420, pickRate: 82 };
    }),
  };
}

function liveGameResponse(url) {
  const gameName = (url.searchParams.get('gameName') ?? '').toLowerCase();
  if (!gameName.includes('rift pilot') && !gameName.includes('live')) {
    return { status: 404, body: { detail: 'not currently in a live game' } };
  }
  return ok(liveGame());
}

function liveGame() {
  const now = Date.now() - 11 * 60 * 1000;
  const blue = [
    participant(100, 266, 'Rift Pilot#NA1', 'live-blue-top', 'DIAMOND', 'II', 84, 54.8, 4, 12),
    participant(100, 64, 'Pathfinder Pro#NA1', 'live-blue-jungle', 'MASTER', 'I', 180, 58.2, 11, 4),
    participant(100, 103, 'Charm Vector#NA1', 'live-blue-mid', 'DIAMOND', 'I', 71, 56.1, 4, 14),
    participant(100, 145, 'Void Marksman#NA1', 'live-blue-bot', 'EMERALD', 'I', 62, 53.4, 4, 7),
    participant(100, 412, 'Lantern Theory#NA1', 'live-blue-support', 'DIAMOND', 'III', 55, 52.2, 4, 3),
  ];
  const red = [
    participant(200, 92, 'Broken Exile#NA1', 'live-red-top', 'DIAMOND', 'III', 51, 50.9, 4, 12),
    participant(200, 62, 'Monkey Tempo#NA1', 'live-red-jungle', 'MASTER', 'I', 210, 57.3, 11, 4),
    participant(200, 157, 'Wind Check#NA1', 'live-red-mid', 'DIAMOND', 'II', 66, 51.7, 4, 14),
    participant(200, 222, 'Zap Rocket#NA1', 'live-red-bot', 'EMERALD', 'II', 48, 52.1, 4, 7),
    participant(200, 89, 'Solar Anchor#NA1', 'live-red-support', 'DIAMOND', 'IV', 38, 49.8, 4, 3),
  ];
  return {
    platform: 'NA1',
    puuid: 'live-blue-top',
    gameId: 990042,
    mapId: 11,
    gameMode: 'CLASSIC',
    gameType: 'MATCHED_GAME',
    gameQueueConfigId: 420,
    gameStartTime: now,
    participants: [...blue, ...red],
  };
}

function participant(teamId, championId, riotId, puuid, tier, division, lp, rankWr, spell1Id, spell2Id) {
  const summary = summaryForChampion(championId);
  return {
    teamId,
    championId,
    spell1Id,
    spell2Id,
    puuid,
    summonerId: `${puuid}-summoner`,
    riotId,
    summonerName: riotId.split('#')[0],
    rank: ranked(tier, division, lp, rankWr),
    championStats: championRecord(championId, summary.role, 42, summary.winRate),
    perks: {
      perkIds: [summary.role === 'MIDDLE' ? 8112 : 8010, 9101, 9104, 8014],
      perkStyle: 8000,
      perkSubStyle: 8100,
    },
    bot: false,
  };
}

function summonerProfileResponse(url) {
  const gameName = url.searchParams.get('gameName') || 'Meta Scout';
  const tagLine = url.searchParams.get('tagLine') || 'NA1';
  const platform = url.searchParams.get('platform') || 'NA1';
  return {
    account: { puuid: 'profile-meta-scout', platform, gameName, tagLine },
    summoner: {
      puuid: 'profile-meta-scout',
      platform,
      profileIconId: 588,
      summonerLevel: 487,
      fetchedAt: '2026-07-01T16:00:00.000Z',
      expiresAt: '2026-07-08T16:00:00.000Z',
      cacheExpiresAt: '2026-07-08T16:00:00.000Z',
    },
    rank: ranked('MASTER', 'I', 247, 56.4),
    summary: {
      puuid: 'profile-meta-scout',
      platform,
      queueId: 420,
      games: 186,
      wins: 106,
      losses: 80,
      kills: 1368,
      deaths: 708,
      assists: 1612,
      avgKills: 7.4,
      avgDeaths: 3.8,
      avgAssists: 8.7,
      kda: 4.21,
      winRate: 57.0,
      firstSeen: '2026-05-19T03:10:00.000Z',
      lastSeen: '2026-07-04T21:48:00.000Z',
    },
    topChampions: [
      championRecord(103, 'MIDDLE', 48, 62.5),
      championRecord(145, 'BOTTOM', 36, 58.3),
      championRecord(64, 'JUNGLE', 29, 55.2),
      championRecord(412, 'UTILITY', 24, 54.2),
      championRecord(266, 'TOP', 22, 50.0),
    ],
    topChampionRoles: [
      championRecord(103, 'MIDDLE', 48, 62.5),
      championRecord(145, 'BOTTOM', 36, 58.3),
      championRecord(64, 'JUNGLE', 29, 55.2),
      championRecord(412, 'UTILITY', 24, 54.2),
      championRecord(266, 'TOP', 22, 50.0),
    ],
    recentMatches: recentMatches(),
    topBuilds: [
      summonerBuild(103, 'MIDDLE', '6672-3006-3031-3094-3036-3072', 42, 61.9),
      summonerBuild(145, 'BOTTOM', '6672-3006-3031-3094-3036-3072', 35, 57.1),
      summonerBuild(64, 'JUNGLE', '6692-3071-3158-6333-3065-3075', 29, 55.2),
      summonerBuild(412, 'UTILITY', '3047-3075-3065-3071-6333-3078', 21, 52.4),
    ],
  };
}

function ranked(tier, division, leaguePoints, winRate) {
  const totalGames = Math.max(40, Math.round(leaguePoints * 0.7) + 86);
  const wins = Math.round(totalGames * (winRate / 100));
  return {
    queueType: 'RANKED_SOLO_5x5',
    tier,
    division,
    rank: division,
    leaguePoints,
    wins,
    losses: totalGames - wins,
    totalGames,
    winRate,
    rankBucket: tier,
    fetchedAt: '2026-07-04T20:00:00.000Z',
    expiresAt: '2026-07-11T20:00:00.000Z',
    rankAvailable: true,
  };
}

function championRecord(championId, role, games, winRate) {
  const wins = Math.round(games * (winRate / 100));
  const losses = games - wins;
  const kills = Math.round(games * 7.1);
  const deaths = Math.round(games * 3.9);
  const assists = Math.round(games * 8.6);
  return {
    queueId: 420,
    championId,
    role,
    games,
    wins,
    losses,
    kills,
    deaths,
    assists,
    avgKills: kills / games,
    avgDeaths: deaths / games,
    avgAssists: assists / games,
    kda: (kills + assists) / Math.max(1, deaths),
    winRate,
  };
}

function summonerBuild(championId, role, signature, games, winRate) {
  const wins = Math.round(games * (winRate / 100));
  const losses = games - wins;
  const kills = Math.round(games * 7.3);
  const deaths = Math.round(games * 3.7);
  const assists = Math.round(games * 8.4);
  const items = signature.split('-');
  return {
    platform: 'NA1',
    queueId: 420,
    championId,
    role,
    finalItemsSignature: signature,
    core2Signature: items.slice(0, 2).join('-'),
    core3Signature: items.slice(0, 3).join('-'),
    runeSignature: '8000|8100|8010-9101-9104-8014|5008-5008-5011',
    spellSignature: role === 'JUNGLE' ? '4-11' : '4-12',
    games,
    wins,
    losses,
    kills,
    deaths,
    assists,
    avgKills: kills / games,
    avgDeaths: deaths / games,
    avgAssists: assists / games,
    kda: (kills + assists) / Math.max(1, deaths),
    winRate,
  };
}

function recentMatches() {
  const championIds = [103, 145, 64, 412, 266, 103, 145, 64, 99, 222, 103, 145];
  return championIds.map((championId, index) => ({
    matchId: `DEMO_${1000 + index}`,
    platform: 'NA1',
    patch,
    queueId: 420,
    championId,
    role: summaryForChampion(championId).role,
    win: index % 3 !== 1,
    kills: 5 + (index % 6),
    deaths: 2 + (index % 4),
    assists: 7 + (index % 8),
    gameStartTimestamp: Date.now() - (index + 1) * 1000 * 60 * 60 * 7,
    durationSeconds: 1580 + index * 58,
  }));
}

function summonerLeaderboardResponse(url) {
  const platform = url.searchParams.get('platform') || 'NA1';
  return {
    platform,
    queueType: 'RANKED_SOLO_5x5',
    results: [
      leaderboardRow(1, 'Rift Pilot', 'NA1', 'CHALLENGER', 'I', 1022, 59.8),
      leaderboardRow(2, 'Meta Scout', 'NA1', 'MASTER', 'I', 247, 56.4),
      leaderboardRow(3, 'Build Oracle', 'NA1', 'MASTER', 'I', 198, 55.7),
      leaderboardRow(4, 'Lane Diff Lab', 'NA1', 'DIAMOND', 'I', 83, 54.1),
      leaderboardRow(5, 'Macro Timer', 'NA1', 'DIAMOND', 'II', 51, 52.8),
    ],
  };
}

function leaderboardRow(rank, gameName, tagLine, tier, division, lp, winRate) {
  const rankRecord = ranked(tier, division, lp, winRate);
  return {
    rank,
    puuid: `leader-${rank}`,
    platform: 'NA1',
    gameName,
    tagLine,
    ranked: rankRecord,
    profileIconId: 588 + rank,
    summonerLevel: 280 + rank * 17,
    rankedGames: rankRecord.totalGames,
    storedGames: 44 + rank * 9,
    storedWins: 28 + rank * 5,
    storedWinRate: 58.3 - rank,
    lastSeenAt: '2026-07-04T20:00:00.000Z',
  };
}

function accountAliasResponse(url) {
  const gameName = url.searchParams.get('gameName') || '';
  const matches = accountAliases().filter((alias) => alias.gameName.toLowerCase().includes(gameName.toLowerCase()));
  if (matches.length === 1) {
    return { status: 'found', ...matches[0] };
  }
  if (matches.length > 1) {
    return { status: 'ambiguous', matches };
  }
  return { status: 'not_found' };
}

function accountAliasSearchResponse(url) {
  const gameName = url.searchParams.get('gameName') || '';
  const limit = Number(url.searchParams.get('limit') ?? 6);
  return {
    matches: accountAliases()
      .filter((alias) => alias.gameName.toLowerCase().includes(gameName.toLowerCase()))
      .slice(0, limit),
  };
}

function accountAliases() {
  return [
    { puuid: 'live-blue-top', platform: 'NA1', gameName: 'Rift Pilot', tagLine: 'NA1' },
    { puuid: 'profile-meta-scout', platform: 'NA1', gameName: 'Meta Scout', tagLine: 'NA1' },
    { puuid: 'demo-build-oracle', platform: 'NA1', gameName: 'Build Oracle', tagLine: 'NA1' },
  ];
}

function winConditionResponse() {
  const script = (id, playerRead) => ({
    id,
    headline: id,
    overview: `${id} overview`,
    matchup: `${id} matchup`,
    ratingRead: `${id} rating`,
    modeRead: `${id} mode`,
    timingRead: `${id} timing`,
    sampleRead: `${id} sample`,
    playerRead,
    facts: ['Secure mid priority before second dragon', 'Avoid splitting tempo after Baron spawns'],
  });
  const evidence = (level = 'Moderate', direction = 'favorable') => ({
    score: 58,
    level,
    direction,
    summary: `${level} ${direction} evidence`,
    wilsonLow: 46,
    wilsonHigh: 68,
    sampleLevel: level.toLowerCase(),
  });
  const blue = teamProfile([266, 64, 103, 145, 412], 'Pick', 'B+');
  const red = teamProfile([92, 62, 157, 222, 89], 'TeamFight', 'B');
  return {
    catalogPatch: version,
    filters: {
      queueId: 420,
      patch,
      rankBucket: '',
      metricSource: 'demo',
      compiledMetricRows: 1200,
      rawTeamRows: 4200,
      filteredTeamRows: 980,
    },
    blue,
    red,
    blueMatchups: [
      conditionMetric('Pick', 'B+', 'primary', 'Primary', 'TeamFight', 'B', true, 62.4, 380, evidence('Strong', 'favorable'), script('pick-teamfight', 'Use Ahri and Thresh fog pressure to force catches before Jinx scales.')),
      conditionMetric('Siege', 'B', 'secondary', 'Secondary', 'TeamFight', 'B', false, 55.8, 260, evidence(), script('siege-teamfight', 'Group around KaiSa item spikes and force short objectives.')),
      conditionMetric('SplitPush', 'C+', 'weak-angle', 'Weak angle', 'Control', 'B-', false, 49.5, 175, evidence('Early', 'mixed'), script('split-control', 'Split only after mid wave is fixed.')),
    ],
    redMatchups: [
      conditionMetric('TeamFight', 'B', 'primary', 'Primary', 'Pick', 'B+', true, 47.6, 380, evidence('Moderate', 'unfavorable'), script('teamfight-pick', 'Red wants grouped fights, but must sweep flanks before engaging.')),
      conditionMetric('Control', 'B-', 'secondary', 'Secondary', 'Pick', 'B+', false, 51.2, 240, evidence(), script('control-pick', 'Hold river setup early and deny Thresh lantern angles.')),
      conditionMetric('SplitPush', 'C', 'weak-angle', 'Weak angle', 'Siege', 'B', false, 44.3, 168, evidence('Early', 'unfavorable'), script('split-siege', 'Avoid isolated side-lane trades into blue pick tools.')),
    ],
  };
}

function teamProfile(championIds, primaryCondition, primaryRating) {
  const axes = [
    { key: 'splitpush', label: 'SplitPush', score: 9, rating: 'C+', deltaFromPrimary: 8, planRole: 'weak-angle', planLabel: 'Weak angle' },
    { key: 'pick', label: 'Pick', score: primaryCondition === 'Pick' ? 18 : 12, rating: primaryCondition === 'Pick' ? primaryRating : 'B-', deltaFromPrimary: primaryCondition === 'Pick' ? 0 : 6, planRole: primaryCondition === 'Pick' ? 'primary' : 'secondary', planLabel: primaryCondition === 'Pick' ? 'Primary' : 'Secondary' },
    { key: 'siege', label: 'Siege', score: 14, rating: 'B', deltaFromPrimary: 4, planRole: 'secondary', planLabel: 'Secondary' },
    { key: 'control', label: 'Control', score: 12, rating: 'B-', deltaFromPrimary: 6, planRole: 'secondary', planLabel: 'Secondary' },
    { key: 'teamfight', label: 'TeamFight', score: primaryCondition === 'TeamFight' ? 17 : 11, rating: primaryCondition === 'TeamFight' ? primaryRating : 'C+', deltaFromPrimary: primaryCondition === 'TeamFight' ? 0 : 7, planRole: primaryCondition === 'TeamFight' ? 'primary' : 'weak-angle', planLabel: primaryCondition === 'TeamFight' ? 'Primary' : 'Weak angle' },
  ];
  return {
    championIds,
    scores: { splitpush: 9, pick: 18, siege: 14, control: 12, teamfight: 17 },
    ratings: Object.fromEntries(axes.map((axis) => [axis.key, axis.rating])),
    axes,
    primaryCondition,
    primaryScore: axes.find((axis) => axis.label === primaryCondition)?.score ?? 18,
    primaryRating,
    primaryMargin: 4,
    sharpness: 'clear',
    sharpnessLabel: 'Clear identity',
    missingChampionIds: [],
  };
}

function conditionMetric(condition, rating, planRole, planLabel, opponentCondition, opponentRating, primary, winRate, games, evidence, script) {
  const wins = Math.round(games * (winRate / 100));
  return {
    condition,
    rating,
    planRole,
    planLabel,
    deltaFromPrimary: primary ? 0 : 4,
    opponentCondition,
    opponentRating,
    opponentPlanRole: primary ? 'primary' : 'secondary',
    opponentPlanLabel: primary ? 'Primary' : 'Secondary',
    primary,
    opponentPrimary: primary,
    wins,
    games,
    winRate,
    confidence: Math.min(90, 40 + games / 12),
    evidence,
    meetsMinGames: true,
    buckets: [
      { bucket: '0-20', wins: Math.round(wins * 0.2), games: Math.round(games * 0.2), winRate: winRate - 2, confidence: 34, meetsMinGames: true },
      { bucket: '20-25', wins: Math.round(wins * 0.22), games: Math.round(games * 0.22), winRate: winRate + 1, confidence: 38, meetsMinGames: true },
      { bucket: '25-30', wins: Math.round(wins * 0.26), games: Math.round(games * 0.26), winRate: winRate + 3, confidence: 42, meetsMinGames: true },
      { bucket: '30-35', wins: Math.round(wins * 0.18), games: Math.round(games * 0.18), winRate: winRate + 0.5, confidence: 36, meetsMinGames: true },
      { bucket: '35+', wins: Math.round(wins * 0.14), games: Math.round(games * 0.14), winRate: winRate - 1, confidence: 30, meetsMinGames: true },
    ],
    script,
  };
}

function summaryForChampion(championId) {
  return guideSummaries.find((row) => row.championId === championId)
    ?? championSummary(championId, 'MIDDLE', 12, 1220, 50.4, 2.7, 1.5, 45.8, 44.2);
}
