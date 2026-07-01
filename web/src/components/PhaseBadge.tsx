const COLORS: Record<string, string> = {
  Pending: 'bg-yellow-500',
  Running: 'bg-primary animate-pulse',
  Succeeded: 'bg-green-500',
  Failed: 'bg-red-500',
  Paused: 'bg-yellow-600',
};

export function PhaseBadge({ phase }: { phase: string }) {
  const color = COLORS[phase] || 'bg-gray-500';
  return <span className={`text-xs px-2 py-1 rounded text-white ${color}`}>{phase}</span>;
}