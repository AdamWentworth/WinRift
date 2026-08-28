import { useEffect, useState } from 'react';
import type { WinConditionAnalysisResponse, WinConditionMetric, WinConditionTeamProfile } from '../../api/types';
import { conditionIconUrl } from '../../lib/winConditions';
import { MetricTile } from '../ui/MetricTile';
import { StatusChip } from '../ui/StatusChip';
import type { TeamSide } from './types';
import { WinConditionLengthChart } from './WinConditionLengthChart';

export function WinConditionPanel({
  analysis,
  yourSide,
  loading,
  error,
}: {
  analysis?: WinConditionAnalysisResponse;
  yourSide: TeamSide;
  loading: boolean;
  error?: string;
}) {
  if (loading) {
    return <section className="win-condition-panel win-condition-state">Win condition metrics loading...</section>;
  }
  if (error) {
    return <section className="win-condition-panel win-condition-state">Win condition metrics unavailable</section>;
  }
  if (!analysis) return null;

  return <WinConditionContent analysis={analysis} yourSide={yourSide} />;
}

function WinConditionContent({ analysis, yourSide }: { analysis: WinConditionAnalysisResponse; yourSide: TeamSide }) {
  const enemySide: TeamSide = yourSide === 'blue' ? 'red' : 'blue';
  const yourTeam = yourSide === 'blue' ? analysis.blue : analysis.red;
  const enemyTeam = enemySide === 'blue' ? analysis.blue : analysis.red;
  const yourMetrics = yourSide === 'blue' ? analysis.blueMatchups : analysis.redMatchups;
  const enemyMetrics = enemySide === 'blue' ? analysis.blueMatchups : analysis.redMatchups;
  const [selectedYourCondition, setSelectedYourCondition] = useState(yourTeam.primaryCondition);
  const [selectedEnemyCondition, setSelectedEnemyCondition] = useState(enemyTeam.primaryCondition);

  useEffect(() => {
    setSelectedYourCondition(yourTeam.primaryCondition);
    setSelectedEnemyCondition(enemyTeam.primaryCondition);
  }, [analysis, yourTeam.primaryCondition, enemyTeam.primaryCondition]);

  const selectedYourMetric = metricForPair(yourMetrics, selectedYourCondition, selectedEnemyCondition) ?? metricForCondition(yourMetrics, selectedYourCondition) ?? primaryMetric(yourMetrics);
  const selectedEnemyMetric = metricForPair(enemyMetrics, selectedEnemyCondition, selectedYourCondition) ?? metricForCondition(enemyMetrics, selectedEnemyCondition) ?? primaryMetric(enemyMetrics);

  return (
    <section className="win-condition-panel" aria-label="Win condition stats">
      <WinConditionSummaryCard
        title="Your Team's Win Condition"
        side={yourSide}
        team={yourTeam}
        metric={selectedYourMetric}
        metrics={yourMetrics}
        selectedCondition={selectedYourMetric?.condition ?? yourTeam.primaryCondition}
        opponentCondition={selectedEnemyMetric?.condition ?? enemyTeam.primaryCondition}
        onSelect={setSelectedYourCondition}
      />
      <WinConditionScriptPanel metric={selectedYourMetric} />
      <WinConditionLengthChart metric={selectedYourMetric} />
      <WinConditionEnemyCard
        side={enemySide}
        team={enemyTeam}
        metric={selectedEnemyMetric}
        metrics={enemyMetrics}
        opponentCondition={selectedYourMetric?.condition ?? yourTeam.primaryCondition}
        onSelect={setSelectedEnemyCondition}
      />
    </section>
  );
}

function WinConditionSummaryCard({
  title,
  side,
  team,
  metric,
  metrics,
  selectedCondition,
  opponentCondition,
  onSelect,
}: {
  title: string;
  side: TeamSide;
  team: WinConditionTeamProfile;
  metric?: WinConditionMetric;
  metrics?: WinConditionMetric[];
  selectedCondition?: string;
  opponentCondition?: string;
  onSelect?: (condition: string) => void;
}) {
  const condition = metric?.condition ?? team.primaryCondition;
  const rating = metric?.rating ?? team.primaryRating;
  const hasSample = Boolean(metric && metric.games > 0);
  const winRateValue = hasSample && metric ? `${metric.winRate.toFixed(2)}%` : '--';
  const strategyValue = hasSample && metric ? (metric.planLabel ?? planLabelFallback(metric)) : 'No sample';
  const gamesValue = hasSample && metric ? String(metric.games) : '0';
  const confidenceValue = hasSample && metric ? (metric.evidence?.level ?? evidenceLevelFallback(metric.games)) : 'No sample';
  return (
    <div className={`legacy-win-card ${side}`}>
      <h2>{title}</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <div className="legacy-win-stat">
        <div className="legacy-win-stat-heading">
          <span>{condition}</span>
          <strong>{rating}</strong>
        </div>
        <div className="legacy-win-stat-grid">
          <MetricTile accent className="legacy-win-stat-tile" label="Win Rate" labelTag="small" value={winRateValue} />
          <MetricTile className="legacy-win-stat-tile" label="Strategy" labelTag="small" value={strategyValue} />
          <MetricTile ariaLabel={`Total Games: ${gamesValue}`} className="legacy-win-stat-tile" label="Games" labelTag="small" value={gamesValue} />
          <MetricTile className="legacy-win-stat-tile" label="Confidence" labelTag="small" value={confidenceValue} />
        </div>
      </div>
      <WinConditionProfileBars team={team} selectedCondition={condition} />
      {metrics && opponentCondition && onSelect ? (
        <WinConditionPlanSwitches
          label="Other Strategies"
          metrics={metrics}
          selectedCondition={selectedCondition ?? condition}
          opponentCondition={opponentCondition}
          primaryCondition={team.primaryCondition}
          onSelect={onSelect}
        />
      ) : null}
    </div>
  );
}

function WinConditionProfileBars({ team, selectedCondition }: { team: WinConditionTeamProfile; selectedCondition: string }) {
  return (
    <div className="win-profile-bars" aria-label="Team win condition profile">
      {team.axes.map((axis) => (
        <div className={`win-profile-row${axis.label === selectedCondition ? ' selected' : ''}`} key={axis.key}>
          <span className="win-profile-name">{axis.label}</span>
          <span className="win-profile-track">
            <span className="win-profile-fill" style={{ width: `${Math.min(100, (axis.score / 25) * 100)}%` }} />
          </span>
          <span className="win-profile-rating">{axis.rating}</span>
        </div>
      ))}
    </div>
  );
}

function WinConditionPlanSwitches({
  label,
  metrics,
  selectedCondition,
  opponentCondition,
  primaryCondition,
  axisOrder,
  allStrategies = false,
  compact = false,
  maxItems = 3,
  showStats = true,
  onSelect,
}: {
  label: string;
  metrics: WinConditionMetric[];
  selectedCondition: string;
  opponentCondition: string;
  primaryCondition: string;
  axisOrder?: string[];
  allStrategies?: boolean;
  compact?: boolean;
  maxItems?: number;
  showStats?: boolean;
  onSelect: (condition: string) => void;
}) {
  const alternatives = allStrategies
    ? allPlanSwitchMetrics(metrics, opponentCondition, axisOrder, selectedCondition)
    : planSwitchMetrics(metrics, selectedCondition, opponentCondition, primaryCondition);
  return (
    <div className={`strategy-switcher${compact ? ' compact' : ''}`}>
      <span className="strategy-switcher-title">{label}</span>
      <div className="strategy-switch-list">
        {alternatives.length > 0 ? (
          alternatives.slice(0, maxItems).map((metric) => (
            <button
              className={`strategy-switch${metric.condition === selectedCondition ? ' selected' : ''}`}
              key={`${metric.condition}-${metric.opponentCondition}`}
              type="button"
              onClick={() => onSelect(metric.condition)}
              aria-current={metric.condition === selectedCondition ? 'true' : undefined}
              aria-label={`Show ${label} ${metric.condition}`}
            >
              <img src={conditionIconUrl(metric.condition)} alt="" />
              <span>
                <strong>{metric.condition} {metric.rating}</strong>
                {showStats ? (
                  <em>{metric.condition === primaryCondition ? 'Primary · ' : ''}{metric.games > 0 ? `${metric.winRate.toFixed(0)}% · ${metric.games}g` : 'No sample'}</em>
                ) : null}
              </span>
            </button>
          ))
        ) : (
          <span className="strategy-switch-empty">{allStrategies ? 'No strategies available' : 'No other strong strategies'}</span>
        )}
      </div>
    </div>
  );
}

function WinConditionScriptPanel({ metric }: { metric?: WinConditionMetric }) {
  const script = metric?.script;
  return (
    <div className="legacy-stats-section match-read-section">
      <h2>Match Read</h2>
      <div className="match-read-copy">
        {script ? (
          <>
            <WinConditionPairStrip metric={metric} />
            <div className="match-read-headline">
              <strong>{script.headline}</strong>
              <span>{planPairRead(metric)}</span>
              <div className="match-read-evidence-row" aria-label="Evidence summary">
                <EvidencePill metric={metric} />
                <span>{metric?.games.toLocaleString() ?? 0} games</span>
                {metric?.evidence?.wilsonLow || metric?.evidence?.wilsonHigh ? (
                  <span>{metric.evidence.wilsonLow.toFixed(1)}-{metric.evidence.wilsonHigh.toFixed(1)} likely range</span>
                ) : null}
              </div>
            </div>
            <p className="match-read-primary">{script.playerRead}</p>
            <div className="match-read-grid">
              <ReadBlock label="Play toward" text={script.modeRead} />
              <ReadBlock label="Watch for" text={script.matchup} />
              <ReadBlock label="Timing" text={script.timingRead} />
              <ReadBlock label="Evidence" text={metric.evidence?.summary ?? script.sampleRead} />
            </div>
            {script.cautionRead ? <p className="match-read-caution">{script.cautionRead}</p> : null}
            <em>{script.sampleRead}</em>
          </>
        ) : (
          <p>Select a win condition pairing to see the match read.</p>
        )}
      </div>
    </div>
  );
}

function WinConditionPairStrip({ metric }: { metric: WinConditionMetric }) {
  return (
    <div className="match-read-pair-strip" aria-label="Selected win condition pairing">
      <span className="pair-side your">
        <img src={conditionIconUrl(metric.condition)} alt="" />
        <strong>Your {metric.condition} {metric.rating}</strong>
      </span>
      <em>vs</em>
      <span className="pair-side enemy">
        <img src={conditionIconUrl(metric.opponentCondition)} alt="" />
        <strong>Enemy {metric.opponentCondition} {metric.opponentRating}</strong>
      </span>
    </div>
  );
}

function EvidencePill({ metric }: { metric?: WinConditionMetric }) {
  const direction = metric?.evidence?.direction ?? 'unknown';
  const score = metric?.evidence?.score ?? 0;
  const level = metric?.evidence?.level ?? 'No sample';
  return (
    <StatusChip className="evidence-pill" tone={direction}>
      Confidence: {level}{score > 0 ? ` ${score.toFixed(0)}/100` : ''}
    </StatusChip>
  );
}

function ReadBlock({ label, text }: { label: string; text: string }) {
  return (
    <div className="match-read-block">
      <span>{label}</span>
      <p>{text}</p>
    </div>
  );
}

function WinConditionEnemyCard({
  side,
  team,
  metric,
  metrics,
  opponentCondition,
  onSelect,
}: {
  side: TeamSide;
  team: WinConditionTeamProfile;
  metric?: WinConditionMetric;
  metrics: WinConditionMetric[];
  opponentCondition: string;
  onSelect: (condition: string) => void;
}) {
  const condition = metric?.condition ?? team.primaryCondition;
  const rating = metric?.rating ?? team.primaryRating;
  return (
    <div className={`legacy-win-card enemy ${side}`}>
      <h2>Enemy Team's Win Condition</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <WinConditionProfileBars team={team} selectedCondition={condition} />
      <div className="enemy-strategy-note">
        <strong>Adjust The Read</strong>
        <span>If the enemy is clearly playing through another strategy, select it below to update the matchup context.</span>
      </div>
      <WinConditionPlanSwitches
        label="Enemy Strategies"
        metrics={metrics}
        selectedCondition={condition}
        opponentCondition={opponentCondition}
        primaryCondition={team.primaryCondition}
        axisOrder={team.axes.map((axis) => axis.label)}
        allStrategies
        compact
        maxItems={4}
        showStats={false}
        onSelect={onSelect}
      />
    </div>
  );
}

function metricForPair(metrics: WinConditionMetric[], condition: string, opponentCondition: string) {
  return metrics.find((metric) => metric.condition === condition && metric.opponentCondition === opponentCondition);
}

function metricForCondition(metrics: WinConditionMetric[], condition: string) {
  return metrics.find((metric) => metric.condition === condition);
}

function primaryMetric(metrics: WinConditionMetric[]) {
  return metrics.find((metric) => metric.primary) ?? metrics[0];
}

function sortedAlternativeMetrics(metrics: WinConditionMetric[]) {
  return [...metrics].sort((a, b) => {
    if (b.winRate !== a.winRate) return b.winRate - a.winRate;
    return b.games - a.games;
  });
}

function uniqueConditionMetrics(metrics: WinConditionMetric[]) {
  const seen = new Set<string>();
  return metrics.filter((metric) => {
    if (seen.has(metric.condition)) return false;
    seen.add(metric.condition);
    return true;
  });
}

function planSwitchMetrics(metrics: WinConditionMetric[], selectedCondition: string, opponentCondition: string, primaryCondition: string) {
  const exactAlternatives = uniqueConditionMetrics(sortedAlternativeMetrics(metrics).filter((metric) => (
    metric.opponentCondition === opponentCondition
    && metric.condition !== selectedCondition
    && isPlayerFacingPlan(metric)
  )));
  if (selectedCondition === primaryCondition) {
    return exactAlternatives.slice(0, 3);
  }

  const primaryReturn = exactAlternatives.find((metric) => metric.condition === primaryCondition)
    ?? metrics.find((metric) => metric.condition === primaryCondition && metric.opponentCondition === opponentCondition)
    ?? metrics.find((metric) => metric.condition === primaryCondition);
  if (!primaryReturn) {
    return exactAlternatives.slice(0, 3);
  }

  const withoutPrimary = exactAlternatives.filter((metric) => metric.condition !== primaryCondition);
  return [primaryReturn, ...withoutPrimary].slice(0, 3);
}

function allPlanSwitchMetrics(metrics: WinConditionMetric[], opponentCondition: string, axisOrder: string[] = [], excludedCondition?: string) {
  const byCondition = new Map<string, WinConditionMetric>();
  metrics
    .filter((metric) => metric.opponentCondition === opponentCondition && metric.condition !== excludedCondition)
    .forEach((metric) => {
      if (!byCondition.has(metric.condition)) {
        byCondition.set(metric.condition, metric);
      }
    });

  const ordered = axisOrder
    .map((condition) => byCondition.get(condition))
    .filter((metric): metric is WinConditionMetric => Boolean(metric));
  const remaining = [...byCondition.values()]
    .filter((metric) => !axisOrder.includes(metric.condition))
    .sort((a, b) => ratingRank(b.rating) - ratingRank(a.rating));
  return [...ordered, ...remaining];
}

function isPlayerFacingPlan(metric: WinConditionMetric) {
  const role = metric.planRole?.toLowerCase();
  if (metric.primary || role === 'primary') return true;
  if (role === 'co-primary' || role === 'strong-secondary') return true;
  if (role === 'secondary') {
    return isRelevantSecondaryPlan(metric);
  }
  if (!role) {
    return metric.primary || isRelevantSecondaryPlan(metric);
  }
  return false;
}

function isRelevantSecondaryPlan(metric: WinConditionMetric) {
  const closeToPrimary = metric.deltaFromPrimary !== undefined && metric.deltaFromPrimary <= 5;
  return closeToPrimary || ratingRank(metric.rating) >= ratingRank('B-');
}

function ratingRank(rating: string) {
  const ratings = ['D-', 'D', 'D+', 'C-', 'C', 'C+', 'B-', 'B', 'B+', 'A-', 'A', 'A+', 'S-', 'S', 'S+'];
  const index = ratings.indexOf(rating);
  return index >= 0 ? index : -1;
}

function evidenceLevelFallback(games: number) {
  if (games <= 0) return 'No sample';
  if (games < 25) return 'Thin';
  if (games < 100) return 'Early';
  if (games < 400) return 'Moderate';
  if (games < 1600) return 'Strong';
  return 'Very strong';
}

function planLabelFallback(metric?: WinConditionMetric) {
  if (!metric) return 'Unknown';
  return metric.primary ? 'Primary' : 'Alternative';
}

function planPairRead(metric: WinConditionMetric) {
  const ownPlan = metric.planLabel ?? planLabelFallback(metric);
  const enemyPlan = metric.opponentPlanLabel ?? (metric.opponentPrimary ? 'Primary' : 'Alternative');
  return `Strategy context: your ${metric.condition} is ${ownPlan.toLowerCase()} into the enemy ${metric.opponentCondition} ${enemyPlan.toLowerCase()}.`;
}

function ratingImageUrl(rating: string) {
  return `/images/win_condition_ratings/${rating}.webp`;
}
