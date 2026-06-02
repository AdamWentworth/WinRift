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
    examples: ['Yorick', 'Camille', 'Jax', 'Tryndamere', 'Fiora'],
  },
  {
    key: 'Pick',
    label: 'Pick',
    shortLabel: 'Catch one target',
    accent: '#f06595',
    plainEnglish: 'Win by finding isolated enemies before a fair fight starts. Pick teams want fog of war, burst damage, hooks, flank angles, and fast collapses.',
    mapPattern: 'Vision denial matters more than standing in lane. A single caught carry can turn into dragon, Baron, towers, or a forced 4v5.',
    teamNeeds: 'Reliable engage or catch, enough burst to finish the target, and teammates close enough to convert the catch into an objective.',
    commonFailure: 'Fishing for catches with no follow-up, face-checking blindly, or letting the game become a clean front-to-back fight.',
    examples: ['Akali', 'LeBlanc', "Kha'Zix", 'Thresh', 'Nocturne'],
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
    examples: ['Ziggs', 'Caitlyn', 'Xerath', 'Jayce', 'Lux'],
  },
  {
    key: 'Control',
    label: 'Control',
    shortLabel: 'Own the space',
    accent: '#72e0ca',
    plainEnglish: 'Win by owning the space where the enemy wants to fight. Control is strongest when protective champions, terrain tools, traps, slows, and peel create a clean pocket for carries like Vayne or Kai\'Sa to keep hitting.',
    mapPattern: 'Arrive first, establish vision, hold choke points, and force the enemy to spend health or cooldowns before they can reach the carry line.',
    teamNeeds: 'A protected damage source plus champions who can deny entry: shields, knockbacks, walls, traps, slows, durable front line, and disciplined objective setup.',
    commonFailure: 'Showing up late, chasing out of controlled space, or drafting protection without a carry who can turn that space into real damage.',
    examples: ['Janna', 'Braum', 'Poppy', 'Vayne', 'Kai\'Sa'],
    carryExamples: ['Vayne', 'Kai\'Sa', 'Kog\'Maw', 'Jinx'],
    protectorExamples: ['Janna', 'Braum', 'Poppy', 'Ivern'],
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
    examples: ['Malphite', 'Orianna', 'Rell', 'Amumu', 'Ornn'],
  },
];

export const WIN_CONDITION_PAGE_ORDER: WinConditionKey[] = ['TeamFight', 'SplitPush', 'Pick', 'Siege', 'Control'];

export function conditionIconUrl(condition: string) {
  return `/images/win_condition_icons/${condition}.png`;
}

export function winConditionDefinition(condition: string) {
  return WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === condition);
}
