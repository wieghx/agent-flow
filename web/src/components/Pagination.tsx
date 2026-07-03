type PaginationProps = {
  page: number;
  totalPages: number;
  totalItems: number;
  rangeStart: number;
  rangeEnd: number;
  onPageChange: (page: number) => void;
  className?: string;
};

function pageNumbers(current: number, total: number): (number | 'ellipsis')[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  const pages = new Set<number>([1, total, current, current - 1, current + 1]);
  const sorted = [...pages].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);
  const out: (number | 'ellipsis')[] = [];
  for (let i = 0; i < sorted.length; i++) {
    if (i > 0 && sorted[i] - sorted[i - 1] > 1) {
      out.push('ellipsis');
    }
    out.push(sorted[i]);
  }
  return out;
}

export function Pagination({
  page,
  totalPages,
  totalItems,
  rangeStart,
  rangeEnd,
  onPageChange,
  className = '',
}: PaginationProps) {
  if (totalItems === 0) return null;

  const nums = pageNumbers(page, totalPages);

  return (
    <div className={`flex flex-wrap items-center justify-between gap-3 text-sm ${className}`}>
      <p className="text-gray-500 text-xs">
        第 {rangeStart}–{rangeEnd} 条，共 {totalItems} 条
      </p>
      <div className="flex items-center gap-1">
        <button
          type="button"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          className="px-2.5 py-1.5 rounded border border-dark-border text-gray-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed"
          aria-label="上一页"
        >
          ‹
        </button>
        {nums.map((n, i) =>
          n === 'ellipsis' ? (
            <span key={`e-${i}`} className="px-1 text-gray-600">
              …
            </span>
          ) : (
            <button
              key={n}
              type="button"
              onClick={() => onPageChange(n)}
              className={`min-w-[2rem] px-2 py-1.5 rounded text-xs ${
                n === page
                  ? 'bg-primary text-white font-medium'
                  : 'border border-dark-border text-gray-400 hover:text-white'
              }`}
            >
              {n}
            </button>
          ),
        )}
        <button
          type="button"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          className="px-2.5 py-1.5 rounded border border-dark-border text-gray-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed"
          aria-label="下一页"
        >
          ›
        </button>
      </div>
    </div>
  );
}