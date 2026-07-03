import { useEffect, useMemo, useState } from 'react';

export type UsePaginationOptions = {
  pageSize?: number;
  /** Reset to page 1 when this value changes (e.g. filter key). */
  resetKey?: string | number;
};

export function usePagination<T>(items: T[], options: UsePaginationOptions = {}) {
  const pageSize = options.pageSize ?? 10;
  const [page, setPage] = useState(1);

  const totalItems = items.length;
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));

  useEffect(() => {
    setPage(1);
  }, [options.resetKey]);

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  const safePage = Math.min(Math.max(page, 1), totalPages);

  const paginatedItems = useMemo(() => {
    const start = (safePage - 1) * pageSize;
    return items.slice(start, start + pageSize);
  }, [items, safePage, pageSize]);

  const rangeStart = totalItems === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const rangeEnd = Math.min(safePage * pageSize, totalItems);

  return {
    page: safePage,
    setPage,
    pageSize,
    totalPages,
    totalItems,
    paginatedItems,
    rangeStart,
    rangeEnd,
  };
}

/** Returns 1-based page index for itemIndex in a paginated list. */
export function pageForIndex(itemIndex: number, pageSize: number): number {
  if (itemIndex < 0) return 1;
  return Math.floor(itemIndex / pageSize) + 1;
}