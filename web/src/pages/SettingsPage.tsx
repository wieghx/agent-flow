import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { fetchAISettings, saveAISettings } from '@/api/client';
import type { AISettingsRole, AISettingsUpdate, AISettingsView, RoleAISettings } from '@/types/api';

const API_KEY_UNCHANGED = '__UNCHANGED__';

const ROLE_META: Record<AISettingsRole, { label: string; icon: string; hint: string }> = {
  planner: {
    label: 'Planner',
    icon: '🧠',
    hint: '对话编排、任务/工作流规划',
  },
  worker: {
    label: 'Worker',
    icon: '⚙️',
    hint: '大纲、章节正文等具体执行',
  },
  monitor: {
    label: 'Monitor',
    icon: '🔍',
    hint: '质量评分与重试决策',
  },
};

function emptyRole(): RoleAISettings {
  return {
    mode: 'remote',
    remote: {
      enabled: true,
      base_url: '',
      model: '',
      temperature: 0.7,
      max_tokens: 8192,
      timeout_seconds: 300,
    },
    local: {
      enabled: false,
      base_url: 'http://localhost:11434',
      model: '',
      temperature: 0.7,
      top_p: 0.9,
      max_tokens: 4096,
      timeout_seconds: 120,
    },
  };
}

function cloneSettings(view: AISettingsView): AISettingsUpdate {
  return {
    planner: structuredClone(view.planner),
    worker: structuredClone(view.worker),
    monitor: structuredClone(view.monitor),
    quality: structuredClone(view.quality),
    retry: structuredClone(view.retry),
  };
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-sm text-gray-300">{label}</span>
      {children}
      {hint ? <span className="text-xs text-gray-500 block">{hint}</span> : null}
    </label>
  );
}

function inputClass() {
  return 'w-full bg-dark-bg border border-dark-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-primary/60';
}

function RolePanel({
  role,
  value,
  onChange,
}: {
  role: AISettingsRole;
  value: RoleAISettings;
  onChange: (next: RoleAISettings) => void;
}) {
  const meta = ROLE_META[role];
  const isRemote = value.mode === 'remote';

  const patch = (partial: Partial<RoleAISettings>) => onChange({ ...value, ...partial });
  const patchRemote = (partial: Partial<RoleAISettings['remote']>) =>
    onChange({ ...value, remote: { ...value.remote, ...partial } });
  const patchLocal = (partial: Partial<RoleAISettings['local']>) =>
    onChange({ ...value, local: { ...value.local, ...partial } });

  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3 pb-4 border-b border-dark-border">
        <span className="text-2xl">{meta.icon}</span>
        <div>
          <h3 className="text-lg font-semibold">{meta.label}</h3>
          <p className="text-sm text-gray-400">{meta.hint}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Field label="运行模式">
          <select
            className={inputClass()}
            value={value.mode}
            onChange={(e) => patch({ mode: e.target.value as RoleAISettings['mode'] })}
          >
            <option value="remote">Remote（远程 API）</option>
            <option value="local">Local（本地 Ollama）</option>
          </select>
        </Field>
        <Field label="描述" hint="可选，仅用于文档说明">
          <input
            className={inputClass()}
            value={value.description || ''}
            onChange={(e) => patch({ description: e.target.value })}
          />
        </Field>
      </div>

      {isRemote ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="Base URL" hint="OpenAI 兼容 API 地址，如 https://api.deepseek.com">
            <input
              className={inputClass()}
              value={value.remote.base_url}
              onChange={(e) => patchRemote({ base_url: e.target.value })}
              placeholder="https://api.deepseek.com"
            />
          </Field>
          <Field
            label="API Key"
            hint={value.remote.api_key_set ? '已配置；留空则保持不变' : '保存后写入 ai_config.local.yaml'}
          >
            <input
              className={inputClass()}
              type="password"
              value={value.remote.api_key || ''}
              onChange={(e) => patchRemote({ api_key: e.target.value })}
              placeholder={value.remote.api_key_set ? '••••••••（已配置）' : 'sk-...'}
            />
          </Field>
          <Field label="模型">
            <input
              className={inputClass()}
              value={value.remote.model}
              onChange={(e) => patchRemote({ model: e.target.value })}
              placeholder="deepseek-chat"
            />
          </Field>
          <Field label="Temperature">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={2}
              step={0.1}
              value={value.remote.temperature}
              onChange={(e) => patchRemote({ temperature: Number(e.target.value) })}
            />
          </Field>
          <Field label="Max Tokens">
            <input
              className={inputClass()}
              type="number"
              min={256}
              step={256}
              value={value.remote.max_tokens}
              onChange={(e) => patchRemote({ max_tokens: Number(e.target.value) })}
            />
          </Field>
          <Field label="超时（秒）">
            <input
              className={inputClass()}
              type="number"
              min={30}
              step={30}
              value={value.remote.timeout_seconds}
              onChange={(e) => patchRemote({ timeout_seconds: Number(e.target.value) })}
            />
          </Field>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="Ollama 地址">
            <input
              className={inputClass()}
              value={value.local.base_url}
              onChange={(e) => patchLocal({ base_url: e.target.value })}
            />
          </Field>
          <Field label="模型">
            <input
              className={inputClass()}
              value={value.local.model}
              onChange={(e) => patchLocal({ model: e.target.value })}
              placeholder="qwen2.5:7b"
            />
          </Field>
          <Field label="Temperature">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={2}
              step={0.1}
              value={value.local.temperature}
              onChange={(e) => patchLocal({ temperature: Number(e.target.value) })}
            />
          </Field>
          <Field label="Top P">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={value.local.top_p}
              onChange={(e) => patchLocal({ top_p: Number(e.target.value) })}
            />
          </Field>
          <Field label="Max Tokens">
            <input
              className={inputClass()}
              type="number"
              min={256}
              step={256}
              value={value.local.max_tokens}
              onChange={(e) => patchLocal({ max_tokens: Number(e.target.value) })}
            />
          </Field>
          <Field label="超时（秒）">
            <input
              className={inputClass()}
              type="number"
              min={30}
              step={30}
              value={value.local.timeout_seconds}
              onChange={(e) => patchLocal({ timeout_seconds: Number(e.target.value) })}
            />
          </Field>
          <Field label="启用本地模式">
            <label className="flex items-center gap-2 text-sm text-gray-300">
              <input
                type="checkbox"
                checked={value.local.enabled}
                onChange={(e) => patchLocal({ enabled: e.target.checked })}
              />
              local.enabled
            </label>
          </Field>
        </div>
      )}

      {role === 'monitor' ? (
        <Field label="System Prompt" hint="Monitor 质量评估的系统提示词">
          <textarea
            className={`${inputClass()} min-h-32 font-mono text-xs leading-relaxed`}
            value={value.system_prompt || ''}
            onChange={(e) => patch({ system_prompt: e.target.value })}
          />
        </Field>
      ) : null}
    </div>
  );
}

export function SettingsPage() {
  const [activeRole, setActiveRole] = useState<AISettingsRole>('planner');
  const [form, setForm] = useState<AISettingsUpdate | null>(null);
  const [meta, setMeta] = useState<{ config_path: string; local_path: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await fetchAISettings();
      setForm(cloneSettings(data));
      setMeta({ config_path: data.config_path, local_path: data.local_path });
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载配置失败');
      setForm({
        planner: emptyRole(),
        worker: emptyRole(),
        monitor: emptyRole(),
        quality: { threshold: 70, max_retries: 3 },
        retry: { max_retries: 3, base_delay_seconds: 5, max_delay_seconds: 60 },
      });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const updateRole = (role: AISettingsRole, next: RoleAISettings) => {
    setForm((prev) => (prev ? { ...prev, [role]: next } : prev));
  };

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);
    setError('');
    setMessage('');
    try {
      const payload: AISettingsUpdate = structuredClone(form);
      for (const role of ['planner', 'worker', 'monitor'] as const) {
        const remote = payload[role].remote;
        if (!remote.api_key?.trim()) {
          remote.api_key = API_KEY_UNCHANGED;
        }
        remote.enabled = payload[role].mode === 'remote';
        if (payload[role].mode === 'local') {
          payload[role].local.enabled = true;
        }
      }
      const saved = await saveAISettings(payload);
      setForm(cloneSettings(saved));
      setMeta({ config_path: saved.config_path, local_path: saved.local_path });
      setMessage('配置已保存并热重载生效');
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading || !form) {
    return (
      <div className="p-5 max-w-4xl mx-auto">
        <p className="text-sm text-gray-400">加载 AI 配置…</p>
      </div>
    );
  }

  return (
    <div className="p-5 max-w-4xl mx-auto space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-xl font-semibold">AI 配置</h2>
          <p className="text-sm text-gray-400 mt-1">
            分别配置 Planner、Worker、Monitor 的模型与 API。保存后写入本地覆盖文件并立即生效。
          </p>
          {meta ? (
            <p className="text-xs text-gray-600 mt-2 font-mono">
              {meta.config_path} → {meta.local_path}
            </p>
          ) : null}
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={load}
            className="text-sm px-3 py-1.5 border border-dark-border rounded-lg hover:bg-dark-bg"
          >
            重新加载
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="text-sm px-4 py-1.5 bg-primary hover:bg-primary-dark rounded-lg disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存配置'}
          </button>
        </div>
      </div>

      {error ? (
        <div className="text-sm text-red-300 bg-red-950/40 border border-red-800/50 rounded-lg px-4 py-3">{error}</div>
      ) : null}
      {message ? (
        <div className="text-sm text-green-300 bg-green-950/30 border border-green-800/40 rounded-lg px-4 py-3">{message}</div>
      ) : null}

      <div className="bg-dark-card border border-dark-border rounded-xl overflow-hidden">
        <div className="flex border-b border-dark-border">
          {(['planner', 'worker', 'monitor'] as const).map((role) => (
            <button
              key={role}
              type="button"
              onClick={() => setActiveRole(role)}
              className={`flex-1 px-4 py-3 text-sm transition-colors ${
                activeRole === role
                  ? 'bg-dark-bg text-white border-b-2 border-primary'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-dark-bg/50'
              }`}
            >
              {ROLE_META[role].icon} {ROLE_META[role].label}
            </button>
          ))}
        </div>
        <div className="p-5">
          <RolePanel role={activeRole} value={form[activeRole]} onChange={(next) => updateRole(activeRole, next)} />
        </div>
      </div>

      <div className="bg-dark-card border border-dark-border rounded-xl p-5 space-y-4">
        <h3 className="text-sm font-semibold text-gray-300 uppercase tracking-wide">全局参数</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Field label="质量阈值（0-100）">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={100}
              value={form.quality.threshold}
              onChange={(e) =>
                setForm((prev) =>
                  prev ? { ...prev, quality: { ...prev.quality, threshold: Number(e.target.value) } } : prev,
                )
              }
            />
          </Field>
          <Field label="质量检查最大重试">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={10}
              value={form.quality.max_retries}
              onChange={(e) =>
                setForm((prev) =>
                  prev ? { ...prev, quality: { ...prev.quality, max_retries: Number(e.target.value) } } : prev,
                )
              }
            />
          </Field>
          <Field label="通用最大重试">
            <input
              className={inputClass()}
              type="number"
              min={0}
              max={10}
              value={form.retry.max_retries}
              onChange={(e) =>
                setForm((prev) =>
                  prev ? { ...prev, retry: { ...prev.retry, max_retries: Number(e.target.value) } } : prev,
                )
              }
            />
          </Field>
          <Field label="重试基础延迟（秒）">
            <input
              className={inputClass()}
              type="number"
              min={1}
              value={form.retry.base_delay_seconds}
              onChange={(e) =>
                setForm((prev) =>
                  prev ? { ...prev, retry: { ...prev.retry, base_delay_seconds: Number(e.target.value) } } : prev,
                )
              }
            />
          </Field>
          <Field label="重试最大延迟（秒）">
            <input
              className={inputClass()}
              type="number"
              min={1}
              value={form.retry.max_delay_seconds}
              onChange={(e) =>
                setForm((prev) =>
                  prev ? { ...prev, retry: { ...prev.retry, max_delay_seconds: Number(e.target.value) } } : prev,
                )
              }
            />
          </Field>
        </div>
      </div>
    </div>
  );
}