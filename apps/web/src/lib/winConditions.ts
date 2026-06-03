export type WinConditionKey = 'SplitPush' | 'Pick' | 'Siege' | 'Control' | 'TeamFight';

export type WinConditionDefinition = {
  key: WinConditionKey;
  label: string;
  shortLabel: string;
  accent: string;
  plainEnglish: string;
  mapPattern: string;
  teamNeeds: string;
  commonFailure: string;
  examples: string[];
  carryExamples?: string[];
  protectorExamples?: string[];
};

export type WinConditionDetail = {
  key: WinConditionKey;
  thesis: string;
  sections: Array<{
    title: string;
    body: string;
  }>;
  signals: string[];
  playPattern: string[];
  failureChecks: string[];
  goodInto: string;
  strugglesInto: string;
  timing: string;
  liveRead: string;
};

export type FlexArchetypeDefinition = {
  key: 'Flex';
  label: string;
  shortLabel: string;
  accent: string;
  plainEnglish: string;
  mapPattern: string;
  teamNeeds: string;
  commonFailure: string;
  examples: string[];
};

export type FlexArchetypeDetail = {
  key: 'Flex';
  thesis: string;
  sections: Array<{
    title: string;
    body: string;
  }>;
  signals: string[];
  playPattern: string[];
  failureChecks: string[];
  goodInto: string;
  strugglesInto: string;
  timing: string;
  liveRead: string;
};

export const WIN_CONDITION_CHAMPION_POOLS: Record<WinConditionKey, string[]> = {
  SplitPush: ['Ambessa', 'Camille', 'Dr. Mundo', 'Fiora', 'Garen', 'Gwen', 'Illaoi', 'Irelia', 'Jax', 'Jayce', 'Kassadin', 'Kled', 'Master Yi', 'Mordekaiser', 'Nasus', 'Olaf', 'Quinn', 'Renekton', 'Riven', 'Sett', 'Shen', 'Shyvana', 'Singed', 'Teemo', 'Trundle', 'Tryndamere', 'Udyr', 'Urgot', 'Yorick'],
  Pick: ['Ahri', 'Akali', 'Bard', "Bel'Veth", 'Blitzcrank', 'Diana', 'Elise', 'Evelynn', 'Fizz', "Kha'Zix", 'LeBlanc', 'Morgana', 'Naafiri', 'Nautilus', 'Nidalee', 'Nocturne', 'Pantheon', 'Pyke', 'Qiyana', 'Rammus', "Rek'Sai", 'Rengar', 'Sett', 'Shaco', 'Skarner', 'Sylas', 'Talon', 'Thresh', 'Twisted Fate', 'Vex', 'Vi', 'Viego', 'Warwick', 'Xin Zhao', 'Zed', 'Zoe'],
  Siege: ['Anivia', 'Ashe', 'Azir', 'Bard', 'Caitlyn', 'Corki', 'Heimerdinger', 'Jayce', 'Jhin', 'Lux', 'Malzahar', 'Mel', 'Milio', 'Seraphine', 'Sivir', 'Sona', 'Soraka', 'Syndra', 'Varus', 'Veigar', "Vel'Koz", 'Xerath', 'Ziggs', 'Zilean'],
  Control: ['Aurelion Sol', 'Braum', "Cho'Gath", 'Fiddlesticks', 'Ivern', 'Janna', 'Jinx', "Kai'Sa", 'Kalista', 'Karma', 'Karthus', 'Kayle', 'Kindred', "Kog'Maw", 'Lissandra', 'Lulu', 'Nami', 'Nilah', 'Nunu & Willump', 'Poppy', 'Renata Glasc', 'Soraka', 'Swain', 'Tahm Kench', 'Taric', 'Tristana', 'Twitch', 'Vayne', 'Yuumi', 'Zeri'],
  TeamFight: ['Aatrox', 'Alistar', 'Amumu', 'Annie', 'Brand', 'Cassiopeia', 'Darius', 'Galio', 'Gnar', 'Gragas', 'Hecarim', 'Karthus', 'Katarina', 'Kennen', 'Leona', 'Lillia', 'Malphite', 'Maokai', 'Miss Fortune', 'Nunu & Willump', 'Orianna', 'Ornn', 'Rakan', 'Rell', 'Rumble', 'Samira', 'Sejuani', 'Sion', 'Smolder', 'Vladimir', 'Wukong', 'Xayah', 'Zac'],
};

export const FLEX_ARCHETYPE_CHAMPION_POOL = ['Akshan', 'Aphelios', 'Aurora', 'Briar', 'Draven', 'Ekko', 'Ezreal', 'Gangplank', 'Graves', 'Hwei', 'Jarvan IV', 'K\'Sante', 'Kayn', 'Lee Sin', 'Lucian', 'Neeko', 'Ryze', 'Senna', 'Taliyah', 'Yasuo', 'Yone', 'Yunara', 'Zaahen'];

export const WIN_CONDITION_DEFINITIONS: WinConditionDefinition[] = [
  {
    key: 'SplitPush',
    label: 'Split Push',
    shortLabel: 'Side-lane pressure',
    accent: '#5db8ff',
    plainEnglish: 'Win by pulling the enemy apart. One or more champions threaten side lanes hard enough that the enemy has to answer, creating space for towers, objectives, or favorable numbers elsewhere.',
    mapPattern: 'Side waves are pushed first, the map stretches, and the team punishes anyone who sends too few people to answer.',
    teamNeeds: 'Dueling, tower threat, escape tools, teleport pressure, and enough waveclear or disengage from the other four players.',
    commonFailure: 'Grouping randomly, fighting 5v5 before side pressure exists, or splitpushing without vision and objective timing.',
    examples: WIN_CONDITION_CHAMPION_POOLS.SplitPush,
  },
  {
    key: 'Pick',
    label: 'Pick',
    shortLabel: 'Catch one target',
    accent: '#b678ff',
    plainEnglish: 'Win by finding isolated enemies before a fair fight starts. Pick teams want fog of war, burst damage, hooks, flank angles, and fast collapses.',
    mapPattern: 'Vision denial matters more than standing in lane. A single caught carry can turn into dragon, Baron, towers, or a forced 4v5.',
    teamNeeds: 'Reliable engage or catch, enough burst to finish the target, and teammates close enough to convert the catch into an objective.',
    commonFailure: 'Fishing for catches with no follow-up, face-checking blindly, or letting the game become a clean front-to-back fight.',
    examples: WIN_CONDITION_CHAMPION_POOLS.Pick,
  },
  {
    key: 'Siege',
    label: 'Siege',
    shortLabel: 'Break structures',
    accent: '#f5c86b',
    plainEnglish: 'Win by taking space around towers and slowly forcing the enemy off structures. Siege teams chip health bars, clear waves, and make defending painful.',
    mapPattern: 'The team sets up early around lanes or objectives, lands poke, controls wave states, and converts health advantages into towers.',
    teamNeeds: 'Long range damage, waveclear, tower pressure, protection from hard engage, and patience to avoid unnecessary all-ins.',
    commonFailure: 'Walking too close, getting engaged on before poke lands, or failing to rotate when the enemy refuses to defend.',
    examples: WIN_CONDITION_CHAMPION_POOLS.Siege,
  },
  {
    key: 'Control',
    label: 'Control',
    shortLabel: 'Own the space',
    accent: '#ff626d',
    plainEnglish: 'Win by owning the space where the enemy wants to fight. Control is strongest when protective champions, terrain tools, traps, slows, and peel create a clean pocket for carries like Vayne or Kai\'Sa to keep hitting.',
    mapPattern: 'Arrive first, establish vision, hold choke points, and force the enemy to spend health or cooldowns before they can reach the carry line.',
    teamNeeds: 'A protected damage source plus champions who can deny entry: shields, knockbacks, walls, traps, slows, durable front line, and disciplined objective setup.',
    commonFailure: 'Showing up late, chasing out of controlled space, or drafting protection without a carry who can turn that space into real damage.',
    examples: WIN_CONDITION_CHAMPION_POOLS.Control,
    carryExamples: ['Vayne', 'Kai\'Sa', 'Kog\'Maw', 'Jinx', 'Zeri', 'Twitch', 'Kayle', 'Karthus', 'Tristana', 'Nilah'],
    protectorExamples: ['Janna', 'Braum', 'Poppy', 'Ivern', 'Lulu', 'Nami', 'Taric', 'Tahm Kench', 'Renata Glasc', 'Yuumi'],
  },
  {
    key: 'TeamFight',
    label: 'Team Fight',
    shortLabel: 'Win grouped fights',
    accent: '#7df06d',
    plainEnglish: 'Win by grouping and taking coordinated fights. Teamfight comps usually have reliable engage, area damage, protection, front line, or scaling damage that shines in 5v5s.',
    mapPattern: 'The team wants objective fights, grouped mid-game contests, and clear engage or counter-engage windows.',
    teamNeeds: 'Cooldown coordination, good target selection, carries positioned to deal damage, and enough durability or peel to survive the first exchange.',
    commonFailure: 'Taking fights before key ultimates are ready, splitting the comp across the map, or starting fights too far from damage dealers.',
    examples: WIN_CONDITION_CHAMPION_POOLS.TeamFight,
  },
];

export const FLEX_ARCHETYPE_DEFINITION: FlexArchetypeDefinition = {
  key: 'Flex',
  label: 'Flex',
  shortLabel: 'Adaptive glue',
  accent: '#d4e3ec',
  plainEnglish: 'Flex is an individual champion identity, not a sixth team win condition. Flex champions are jack-of-all-trades pieces that can help a draft pivot between plans when the map state changes.',
  mapPattern: 'The map pattern is chosen by the game state: early tempo, which lane is ahead, objective timing, and which enemy weakness is easiest to attack.',
  teamNeeds: 'A Flex champion needs teammates who still create a real pressure point. Flex adds options; it does not replace the need for damage, engage, wave control, or objective setup.',
  commonFailure: 'A team full of adaptable pieces can become vague if nobody commits to the moment. Flex turns bad when every player can do a little, but nobody creates a clear way to win.',
  examples: FLEX_ARCHETYPE_CHAMPION_POOL,
};

export const WIN_CONDITION_PAGE_ORDER: WinConditionKey[] = ['TeamFight', 'SplitPush', 'Pick', 'Siege', 'Control'];

export const WIN_CONDITION_DETAILS: Record<WinConditionKey, WinConditionDetail> = {
  TeamFight: {
    key: 'TeamFight',
    thesis: 'Team Fight is the cleanest version of "we win when everyone shows up together." The composition wants reliable engage, area damage, peel, front line, or scaling damage that gets stronger when ten champions collide around objectives.',
    sections: [
      { title: 'What It Is', body: 'A team-fight identity means the comp gains value from coordinated cooldowns and grouped positioning. One champion may start the fight, another may protect the carries, and another may deliver the damage, but the plan only becomes real when those pieces overlap.' },
      { title: 'How It Wins', body: 'Team-fight teams usually convert dragons, Barons, and mid-game contests. They want to force the enemy to answer a grouped threat, then punish with engage, counter-engage, layered ultimates, or front-to-back damage.' },
      { title: 'How To Read It', body: 'Look for champions with large-area ultimates, durable front lines, reliable engage, or carries that thrive when protected. A single team-fight champion does not make a team-fight comp; the question is whether the team can arrive together and press cooldowns in a useful order.' },
    ],
    signals: ['Reliable engage or counter-engage', 'Multiple area-damage or area-control spells', 'Carries that scale in grouped fights', 'Front line or peel that keeps damage alive'],
    playPattern: ['Track major ultimates before fighting', 'Group around dragon, Baron, or mid waves', 'Force the enemy into narrow terrain', 'Fight with damage dealers close enough to follow engage'],
    failureChecks: ['Starting fights too far from carries', 'Forcing without key ultimates', 'Splitting side lanes while the enemy groups first', 'Engaging into better terrain or stronger cooldowns'],
    goodInto: 'Often feels strongest against comps that must walk into you or cannot punish grouped positioning before the fight starts.',
    strugglesInto: 'Can struggle when split push pulls members away, siege lowers health before engage, or pick removes a key member before the 5v5 exists.',
    timing: 'Power often spikes when ultimate ranks, core defensive items, or two-item carry timings line up around objectives.',
    liveRead: 'On the live page, a strong Team Fight grade means the comp is likely easier to execute when it groups with cooldowns ready. It is still not a command to fight every timer.',
  },
  SplitPush: {
    key: 'SplitPush',
    thesis: 'Split Push wins by making the enemy answer the side lane. The point is not wandering alone; it is creating a map problem the enemy cannot solve without losing towers, neutral objectives, or numbers elsewhere.',
    sections: [
      { title: 'What It Is', body: 'A split-push identity usually has at least one champion that can threaten towers, win or survive side-lane duels, and escape or waste time when multiple enemies respond.' },
      { title: 'How It Wins', body: 'The team stretches the map. If one defender answers, the split pusher threatens a kill or tower. If two defenders answer, the rest of the team can pressure Baron, dragon, mid tower, or vision.' },
      { title: 'How To Read It', body: 'Look for side-lane duelists, teleport pressure, tower damage, mobility, and a four-player group that can avoid dying while the side lane works. The non-splitting teammates matter as much as the split pusher.' },
    ],
    signals: ['One or more strong side-lane duelists', 'Tower threat and wave control', 'Escape tools or ability to waste time', 'Four-player waveclear or disengage elsewhere'],
    playPattern: ['Set side waves before objectives spawn', 'Avoid grouped fights until pressure is established', 'Trade map space when the enemy over-answers', 'Keep vision deep enough to leave before collapse'],
    failureChecks: ['Splitting with no objective to trade', 'Dying before the wave reaches tower', 'The four-player group getting engaged on', 'Grouping randomly and losing the side-lane advantage'],
    goodInto: 'Often punishes slower team-fight or control comps that need all five champions together to function.',
    strugglesInto: 'Can struggle against fast pick comps, global pressure, or teams that can force Baron before side pressure matters.',
    timing: 'Often spikes when the side-lane champion gets first or second item and can demand more than one answer.',
    liveRead: 'A strong Split Push read means the team has credible side-lane pressure. It does not mean every player should avoid grouping; the trade has to exist first.',
  },
  Pick: {
    key: 'Pick',
    thesis: 'Pick wins before the fair fight starts. The comp wants fog of war, crowd control, burst, flank threat, and fast objective conversion after one enemy gets caught.',
    sections: [
      { title: 'What It Is', body: 'A pick identity is built around removing a target from the map or forcing a recall before the enemy can form a clean setup. Hooks, charms, burst assassins, point-and-click lockdown, and flank pressure all contribute.' },
      { title: 'How It Wins', body: 'Pick teams turn one mistake into an objective. A caught support can become Baron vision. A caught carry can become dragon. A caught side-laner can become a tower and deep wards.' },
      { title: 'How To Read It', body: 'Look for champions that threaten isolated targets and teammates that can arrive quickly. Pick without follow-up is just fishing; pick with conversion is a win condition.' },
    ],
    signals: ['Reliable catch tools', 'Burst damage or fast collapse', 'Fog-of-war control', 'Mobility, flank threat, or long-range engage'],
    playPattern: ['Deny vision before objectives', 'Stand where enemies must walk, not where they already see you', 'Collapse quickly after crowd control lands', 'Convert kills into Baron, dragon, towers, or resets'],
    failureChecks: ['Fishing hooks with no follow-up', 'Face-checking before vision is denied', 'Letting the enemy group front-to-back', 'Using pick tools on tanks with no objective reward'],
    goodInto: 'Often punishes siege, split, or greedy scaling comps that need time and map access to set up.',
    strugglesInto: 'Can struggle against durable control shells, clean grouped team-fight comps, or teams with enough vision and peel to deny isolation.',
    timing: 'Often spikes in mid game when support vision, first lethality/AP items, and mobility tools let the team control fog.',
    liveRead: 'A strong Pick read says the comp can create uneven fights. It is context for vision and positioning, not a license to coin-flip every brush.',
  },
  Siege: {
    key: 'Siege',
    thesis: 'Siege wins by making defense miserable. The comp wants range, poke, waveclear, tower pressure, and enough safety that the enemy cannot simply engage before health bars are low.',
    sections: [
      { title: 'What It Is', body: 'A siege identity is about controlled pressure around structures and objectives. The team chips health bars, clears waves, threatens towers, and makes the enemy choose between giving space or engaging at a disadvantage.' },
      { title: 'How It Wins', body: 'Siege teams build health and tempo advantages before committing. They rotate between towers and objectives, land poke, and convert the enemy retreat into plates, towers, dragon setup, or Baron control.' },
      { title: 'How To Read It', body: 'Look for long-range damage, safe waveclear, traps, tower damage, and disengage. Siege is strongest when the team has patience and enough protection to avoid hard engage.' },
    ],
    signals: ['Long-range poke or zone control', 'Strong waveclear', 'Tower pressure', 'Disengage or peel against hard engage'],
    playPattern: ['Arrive early and control the wave', 'Chip health before committing', 'Rotate when defenders abandon a tower', 'Keep spacing so engage tools cannot hit multiple carries'],
    failureChecks: ['Walking too close before poke lands', 'Sieging without wave control', 'Ignoring flanks', 'Overstaying after cooldowns are spent'],
    goodInto: 'Often pressures control or scaling comps that need time and position before they can fight.',
    strugglesInto: 'Can struggle against hard engage, flank-heavy team fight, or pick comps that punish careless setup.',
    timing: 'Often spikes when poke champions complete first or second items and when outer towers are still available to pressure.',
    liveRead: 'A strong Siege read says the comp may win by creating health advantages before fights. It does not mean standing mid forever if side waves and vision are bad.',
  },
  Control: {
    key: 'Control',
    thesis: 'Control wins by owning the space the enemy needs to enter. It is strongest when protective champions and terrain tools create a safe pocket for carries like Vayne, Kai\'Sa, Kog\'Maw, Jinx, or Zeri.',
    sections: [
      { title: 'What It Is', body: 'A control identity is not just defensive. It uses shields, slows, traps, knockbacks, walls, front line, and vision to make enemy movement expensive while a protected damage source keeps hitting.' },
      { title: 'How It Wins', body: 'Control teams arrive first, hold choke points, and force the enemy to spend health or cooldowns to enter. If the enemy dives, peel turns that dive into a losing fight. If the enemy waits, the controlled objective gets taken.' },
      { title: 'How To Read It', body: 'Look for the pairing: a real damage source plus champions that buy space. Protection without damage can stall but not win; damage without protection may not survive long enough to matter.' },
    ],
    signals: ['Protected late-game or high-output carries', 'Peel, shields, knockbacks, slows, or traps', 'Front line that can hold space', 'Objective setup and vision control'],
    playPattern: ['Arrive first to objectives', 'Hold choke points rather than chase away from them', 'Keep carries inside the protected pocket', 'Force enemies to walk through controlled terrain'],
    failureChecks: ['Showing up late', 'Chasing out of controlled space', 'Drafting peel without enough damage', 'Letting divers bypass the front line for free'],
    goodInto: 'Often punishes dive, pick, and short-range team-fight comps when the control team is set up first.',
    strugglesInto: 'Can struggle against split push that pulls the setup apart or siege that lowers health before control is established.',
    timing: 'Often becomes clearer once the protected carry has enough items to punish enemies walking into the zone.',
    liveRead: 'A strong Control read means the comp likely wants to set the battlefield first. If the team chases out of that space, the rating may stop matching reality.',
  },
};

export const FLEX_ARCHETYPE_DETAIL: FlexArchetypeDetail = {
  key: 'Flex',
  thesis: 'Flex champions do not say "we only win this one way." They are adaptive pieces that can bridge plans, hide draft intent, and help the team lean toward whichever pressure point the game creates.',
  sections: [
    { title: 'What It Is', body: 'A Flex identity means the champion has a flatter strategic profile. Lee Sin, Ezreal, Hwei, Jarvan IV, Kayn, Senna, and similar champions can contribute to more than one plan without hard-locking the team into Split Push, Pick, Siege, Control, or Team Fight by themselves.' },
    { title: 'How It Wins', body: 'Flex wins by making the team less brittle. If the carry gets ahead, the Flex champion can protect or enable that lane. If the enemy mispositions, it can become a pick tool. If the game slows down, it can help waveclear, contest vision, or bridge into grouped fights.' },
    { title: 'How To Read It', body: 'Do not read Flex as "no identity." Read it as "watch the first few map decisions." Flex-heavy teams often reveal their real plan through lane priority, first objective setup, who gets resources, and which cooldowns they use proactively.' },
  ],
  signals: ['Balanced scores across several win-condition axes', 'Tools that can start, follow, peel, poke, or skirmish depending on the game', 'Champions that fit several lanes, roles, builds, or tempo plans', 'Drafts where one champion helps connect the main plan to a secondary plan'],
  playPattern: ['Identify which lane or champion is actually ahead', 'Use the Flex piece to reinforce that pressure point', 'Pivot when the enemy overcommits to stopping the first plan', 'Resolve into a concrete plan before late-game fights become mandatory'],
  failureChecks: ['Nobody creates a reliable pressure point', 'The team keeps changing plans after every small setback', 'The Flex champion spends cooldowns reactively instead of enabling the strongest teammate', 'The draft has many options but no damage, engage, waveclear, or scaling anchor'],
  goodInto: 'Often useful into rigid comps that reveal their plan early, because Flex can choose whether to answer with pick, peel, side pressure, skirmish, or grouped fighting.',
  strugglesInto: 'Can struggle into comps with one overwhelming identity and clean execution, especially when the Flex side never commits to its own best lane, timing, or objective setup.',
  timing: 'Flex value is often highest in early and mid game, when tempo choices decide which plan becomes real. Later, the team usually needs to resolve Flex into a clear fight, pick, siege, control, or side-lane pattern.',
  liveRead: 'On the live page, Flex should be treated as an individual champion archetype. It can explain why a champion feels useful in several plans, but it should not replace the team-level win-condition read.',
};

export const WIN_CONDITION_ROUTE_SLUGS: Record<WinConditionKey, string> = {
  TeamFight: 'teamfight',
  SplitPush: 'splitpush',
  Pick: 'pick',
  Siege: 'siege',
  Control: 'control',
};

export function conditionIconUrl(condition: string) {
  return `/images/win_condition_icons/${condition}.png`;
}

export function winConditionDefinition(condition: string) {
  return WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === condition);
}

export function winConditionDetail(condition: WinConditionKey) {
  return WIN_CONDITION_DETAILS[condition];
}

export function winConditionSlug(condition: WinConditionKey) {
  return WIN_CONDITION_ROUTE_SLUGS[condition];
}

export function winConditionPath(condition: WinConditionKey) {
  return `/${winConditionSlug(condition)}`;
}

export function flexArchetypePath() {
  return '/flex';
}

export function winConditionFromSlug(slug: string | undefined): WinConditionKey | undefined {
  if (!slug) return undefined;
  const normalized = slug.toLowerCase().replace(/[\s_-]/g, '');
  return (Object.keys(WIN_CONDITION_ROUTE_SLUGS) as WinConditionKey[])
    .find((key) => WIN_CONDITION_ROUTE_SLUGS[key] === normalized);
}
