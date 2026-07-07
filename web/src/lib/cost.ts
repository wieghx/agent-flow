export function formatCostUSD(amount?: number): string {
  if (amount == null || !Number.isFinite(amount) || amount <= 0) {
    return '$0.00';
  }
  if (amount < 0.01) {
    return `<$0.01`;
  }
  return `$${amount.toFixed(2)}`;
}