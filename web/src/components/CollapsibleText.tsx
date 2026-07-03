import { useEffect, useMemo, useState } from 'react';

const CHAR_THRESHOLD = 280;
const LINE_THRESHOLD = 8;

type CollapsibleTextProps = {
  text: string;
  /** Latest / active message stays expanded when long. */
  expandedByDefault?: boolean;
  /** Bubble background for fade gradient. */
  variant?: 'card' | 'primary';
};

function isLongText(text: string): boolean {
  return text.length > CHAR_THRESHOLD || text.split('\n').length > LINE_THRESHOLD;
}

export function CollapsibleText({
  text,
  expandedByDefault = false,
  variant = 'card',
}: CollapsibleTextProps) {
  const long = useMemo(() => isLongText(text), [text]);
  const [expanded, setExpanded] = useState(() => expandedByDefault || !long);

  useEffect(() => {
    setExpanded(expandedByDefault || !long);
  }, [text, expandedByDefault, long]);

  if (!long) {
    return <>{text}</>;
  }

  const fadeFrom = variant === 'primary' ? 'from-primary' : 'from-dark-card';

  return (
    <div>
      <div className={expanded ? undefined : 'relative max-h-28 overflow-hidden'}>
        {text}
        {!expanded && (
          <div
            className={`pointer-events-none absolute inset-x-0 bottom-0 h-12 bg-gradient-to-t ${fadeFrom} to-transparent`}
            aria-hidden
          />
        )}
      </div>
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className={`mt-2 text-xs hover:underline ${
          variant === 'primary' ? 'text-white/80 hover:text-white' : 'text-primary'
        }`}
      >
        {expanded ? '收起' : `展开全文（${text.length} 字）`}
      </button>
    </div>
  );
}