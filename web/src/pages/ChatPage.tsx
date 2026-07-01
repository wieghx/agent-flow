import { useEffect, useRef, useState } from 'react';
import { fetchConversation, sendChat } from '@/api/client';
import type { ConversationMessage } from '@/types/api';

interface ChatMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

function messageContent(m: ConversationMessage & { Content?: string }): string {
  return (m.content || m.Content || '').trim();
}

function messageRole(m: ConversationMessage & { Role?: string }): ChatMessage['role'] {
  const role = m.role || m.Role || 'system';
  return role === 'user' || role === 'assistant' || role === 'system' ? role : 'system';
}

/** 工作流逐章任务反馈已在后端过滤；历史数据在此隐藏 */
function shouldShowMessage(m: ConversationMessage & { Content?: string }): boolean {
  const content = messageContent(m);
  if (!content) return false;
  if (/✅ 任务「wf-[^」]+-(chapter|arc)-/.test(content)) return false;
  return true;
}

function toChatMessages(messages: ConversationMessage[]): ChatMessage[] {
  return messages
    .filter(shouldShowMessage)
    .map((m) => ({
      role: messageRole(m),
      content: messageContent(m),
    }));
}

export function ChatPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const lastUpdatedRef = useRef('');

  const syncFromServer = async () => {
    const conv = await fetchConversation();
    if (!conv?.messages?.length) return;
    if (conv.updated_at && conv.updated_at === lastUpdatedRef.current) return;
    lastUpdatedRef.current = conv.updated_at || '';
    setMessages(toChatMessages(conv.messages));
  };

  useEffect(() => {
    syncFromServer();
    const id = window.setInterval(syncFromServer, 5000);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setInput('');
    setMessages((m) => [...m, { role: 'user', content: text }]);
    setSending(true);
    try {
      const data = await sendChat(text);
      setMessages((m) => [...m, { role: 'assistant', content: data.assistant_reply }]);
      if (data.task) {
        setMessages((m) => [
          ...m,
          { role: 'system', content: `📋 待批准任务已创建，请在侧栏批准。` },
        ]);
      }
      if (data.workflow_suggested && data.workflow) {
        const chapters = data.workflow.params?.chapterCount || '?';
        setMessages((m) => [
          ...m,
          {
            role: 'system',
            content: `📚 待批准工作流：${data.workflow?.description || '长篇小说'}（${chapters} 章），请在侧栏批准。`,
          },
        ]);
      }
      await syncFromServer();
    } catch (e) {
      setMessages((m) => [
        ...m,
        { role: 'system', content: `错误: ${e instanceof Error ? e.message : String(e)}` },
      ]);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="h-full flex flex-col max-w-4xl mx-auto p-5">
      <div className="flex-1 overflow-y-auto space-y-4 mb-4">
        {messages.length === 0 && (
          <p className="text-center text-gray-500 py-16">
            试试：「帮我写一本 100 章的荒岛生存小说」
          </p>
        )}
        {messages.map((msg, i) => (
          <div
            key={`${msg.role}-${i}`}
            className={`animate-fade-in flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : ''}`}
          >
            <div
              className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 ${
                msg.role === 'user' ? 'bg-primary' : msg.role === 'assistant' ? 'bg-green-600' : 'bg-gray-600'
              }`}
            >
              {msg.role === 'user' ? '👤' : msg.role === 'assistant' ? '🧠' : 'ℹ️'}
            </div>
            <div
              className={`rounded-xl px-4 py-3 max-w-[85%] whitespace-pre-wrap ${
                msg.role === 'user'
                  ? 'bg-primary'
                  : 'bg-dark-card border border-dark-border text-gray-100'
              }`}
            >
              {msg.content}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <div className="flex gap-3">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
          rows={2}
          placeholder="输入消息，例如：帮我写一本 100 章的荒岛生存小说..."
          className="flex-1 bg-dark-card border border-dark-border rounded-xl px-4 py-3 resize-none focus:outline-none focus:border-primary"
        />
        <button
          type="button"
          onClick={send}
          disabled={sending}
          className="bg-primary hover:bg-primary-dark px-6 py-3 rounded-xl font-medium disabled:opacity-50"
        >
          {sending ? '...' : '发送'}
        </button>
      </div>
    </div>
  );
}