import { useCallback, useEffect, useRef, useState } from 'react';
import type { TaskEvent } from '@/types/api';

export interface SSELogEntry {
  id: string;
  step: string;
  message: string;
  at: string;
}

export function useTaskSSE() {
  const [connectedTask, setConnectedTask] = useState<string | null>(null);
  const [status, setStatus] = useState('未连接');
  const [events, setEvents] = useState<SSELogEntry[]>([]);
  const sourceRef = useRef<EventSource | null>(null);

  const disconnect = useCallback(() => {
    sourceRef.current?.close();
    sourceRef.current = null;
    setConnectedTask(null);
    setStatus('未连接');
  }, []);

  const connect = useCallback((taskName: string, namespace = 'default') => {
    disconnect();
    const url = `/tasks/events?namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(taskName)}`;
    const source = new EventSource(url);
    sourceRef.current = source;
    setConnectedTask(taskName);
    setStatus(`连接中: ${taskName}`);

    const push = (step: string, raw: string) => {
      let message = raw;
      try {
        const parsed = JSON.parse(raw) as TaskEvent;
        message = parsed.message || parsed.step || raw;
        if (parsed.score) message += ` (score=${parsed.score})`;
        if (parsed.retry !== undefined) message += ` [retry=${parsed.retry}]`;
      } catch {
        /* plain text */
      }
      setEvents((prev) => {
        const next = [
          ...prev,
          { id: `${Date.now()}-${prev.length}`, step, message, at: new Date().toLocaleTimeString() },
        ];
        return next.slice(-200);
      });
    };

    source.addEventListener('connected', () => setStatus(`已连接: ${taskName}`));
    source.addEventListener('ping', () => {});
    ['sandbox', 'worker', 'monitor', 'succeeded', 'failed'].forEach((step) => {
      source.addEventListener(step, (ev) => push(step, (ev as MessageEvent).data));
    });
    source.onerror = () => {
      setStatus(`连接中断: ${taskName}`);
      push('error', 'SSE 连接中断');
    };
  }, [disconnect]);

  useEffect(() => () => disconnect(), [disconnect]);

  return { connectedTask, status, events, connect, disconnect, clearEvents: () => setEvents([]) };
}