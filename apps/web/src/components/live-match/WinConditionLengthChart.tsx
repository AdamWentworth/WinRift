import type { WinConditionMetric } from '../../api/types';

export function WinConditionLengthChart({ metric }: { metric?: WinConditionMetric }) {
  const buckets = metric?.buckets ?? [];
  const points = buckets.map((bucket, index) => ({
    x: buckets.length <= 1 ? 50 : 12 + (index * 76) / (buckets.length - 1),
    y: chartY(bucket.winRate),
    bucket,
  })).filter((point) => point.bucket.games > 0);
  const pointString = points.map((point) => `${point.x},${point.y}`).join(' ');
  const areaPath = chartAreaPath(points);
  return (
    <div className="legacy-stats-section chart-section">
      <h2>Winrate By Game Length</h2>
      <div className="chart-shell">
        <div className="chart-plot">
          <svg className="winrate-chart" viewBox="0 0 100 100" role="img" aria-label="Winrate by game length from 35% to 65%" preserveAspectRatio="none">
            <defs>
              <linearGradient id="durationWinrateArea" x1="0" x2="0" y1="0" y2="1">
                <stop offset="0%" stopColor="#7df4dd" stopOpacity="0.28" />
                <stop offset="58%" stopColor="#4bc0c0" stopOpacity="0.11" />
                <stop offset="100%" stopColor="#ff7979" stopOpacity="0.08" />
              </linearGradient>
            </defs>
            <line className="chart-gridline" x1="10" y1={chartY(65)} x2="92" y2={chartY(65)} />
            <line className="chart-gridline chart-baseline" x1="10" y1={chartY(50)} x2="92" y2={chartY(50)} />
            <line className="chart-gridline" x1="10" y1={chartY(35)} x2="92" y2={chartY(35)} />
            {areaPath ? <path className="chart-area" d={areaPath} /> : null}
            {pointString ? <polyline className="chart-line" points={pointString} /> : null}
          </svg>
          <div className="chart-y-axis" aria-hidden="true">
            <span style={{ top: `${chartY(65)}%` }}>65%</span>
            <span className="chart-y-axis-baseline" style={{ top: `${chartY(50)}%` }}>50%</span>
            <span style={{ top: `${chartY(35)}%` }}>35%</span>
          </div>
          {points.map((point) => (
            <span
              className={`chart-marker ${chartPointClass(point.bucket.winRate)}`}
              key={point.bucket.bucket}
              style={{ left: `${point.x}%`, top: `${point.y}%` }}
              title={`${point.bucket.bucket}: ${point.bucket.winRate.toFixed(1)}% over ${point.bucket.games} games`}
            >
              <span className="chart-marker-value">{point.bucket.winRate.toFixed(0)}%</span>
            </span>
          ))}
          {!points.length ? <div className="chart-no-samples">No duration samples</div> : null}
        </div>
        <div className="chart-labels">
          {buckets.map((bucket) => (
            <span
              className={`${bucket.meetsMinGames ? '' : 'thin-sample'} ${bucket.games > 0 ? chartPointClass(bucket.winRate) : ''}`.trim()}
              key={bucket.bucket}
            >
              <b>{bucket.bucket}</b>
              <strong>{bucket.games > 0 ? `${bucket.winRate.toFixed(0)}%` : '--'}</strong>
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

const chartMinWinRate = 35;
const chartMaxWinRate = 65;
const chartTop = 8;
const chartBottom = 92;

function chartY(winRate: number) {
  const clamped = Math.max(chartMinWinRate, Math.min(chartMaxWinRate, winRate));
  const progress = (clamped - chartMinWinRate) / (chartMaxWinRate - chartMinWinRate);
  return chartBottom - progress * (chartBottom - chartTop);
}

function chartAreaPath(points: { x: number; y: number }[]) {
  if (!points.length) return '';
  const first = points[0];
  const last = points[points.length - 1];
  const line = points.map((point) => `L ${point.x} ${point.y}`).join(' ');
  return `M ${first.x} ${chartBottom} ${line} L ${last.x} ${chartBottom} Z`;
}

function chartPointClass(winRate: number) {
  if (winRate > 50) return 'chart-point-favorable';
  if (winRate < 50) return 'chart-point-unfavorable';
  return 'chart-point-even';
}
