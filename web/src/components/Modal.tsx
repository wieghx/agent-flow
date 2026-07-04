import type { ReactNode } from 'react';

export function Modal({
  title,
  open,
  onClose,
  children,
  wide = false,
  footer,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  wide?: boolean;
  footer?: ReactNode;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div
        className={`bg-dark-card border border-dark-border rounded-xl w-full max-h-[85vh] overflow-hidden flex flex-col ${
          wide ? 'max-w-5xl' : 'max-w-3xl'
        }`}
      >
        <div className="p-5 border-b border-dark-border flex items-center justify-between">
          <h3 className="text-lg font-semibold">{title}</h3>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-white text-2xl leading-none">
            ×
          </button>
        </div>
        <div className="p-5 overflow-y-auto">{children}</div>
        <div className="p-4 border-t border-dark-border flex justify-end gap-2">
          {footer ?? (
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 bg-dark-bg border border-dark-border rounded-lg hover:border-primary"
            >
              关闭
            </button>
          )}
        </div>
      </div>
    </div>
  );
}