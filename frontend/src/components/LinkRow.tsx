import { FormEvent, useState } from "react";
import { api, ApiError, Link, LinkUpdate, shortUrl } from "../api";

interface Props {
  link: Link;
  onChanged: (link: Link) => void;
  onDeleted: (id: string) => void;
}

const dateFormat = new Intl.DateTimeFormat("ru-RU", { dateStyle: "short", timeStyle: "short" });

export function LinkRow({ link, onChanged, onDeleted }: Props) {
  const [editing, setEditing] = useState(false);
  const [url, setUrl] = useState(link.originalUrl);
  const [code, setCode] = useState(link.code);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);

  const short = shortUrl(link.code);

  function startEdit() {
    setUrl(link.originalUrl);
    setCode(link.code);
    setError(null);
    setEditing(true);
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    const update: LinkUpdate = {};
    if (url.trim() !== link.originalUrl) update.originalUrl = url.trim();
    if (code.trim() !== link.code) update.code = code.trim();
    if (Object.keys(update).length === 0) {
      setEditing(false);
      return;
    }

    setBusy(true);
    setError(null);
    try {
      onChanged(await api.update(link.id, update));
      setEditing(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!window.confirm(`Удалить ссылку /r/${link.code}?`)) return;
    setBusy(true);
    try {
      await api.remove(link.id);
      onDeleted(link.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
      setBusy(false);
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(short);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      window.prompt("Скопируйте ссылку", short);
    }
  }

  if (editing) {
    return (
      <li className="card link-row editing">
        <form onSubmit={save} className="edit-form">
          <label className="field">
            <span>Код</span>
            <input value={code} onChange={(e) => setCode(e.target.value)} pattern="[A-Za-z0-9_\-]{3,32}" required />
          </label>
          <label className="field field-url">
            <span>Длинная ссылка</span>
            <input type="url" value={url} onChange={(e) => setUrl(e.target.value)} required />
          </label>
          <div className="actions">
            <button type="submit" className="primary" disabled={busy}>
              Сохранить
            </button>
            <button type="button" onClick={() => setEditing(false)} disabled={busy}>
              Отмена
            </button>
          </div>
        </form>
        {error && <p className="error" role="alert">{error}</p>}
      </li>
    );
  }

  return (
    <li className="card link-row">
      <div className="link-main">
        <a className="short" href={short} target="_blank" rel="noreferrer">
          /r/{link.code}
        </a>
        <a className="original" href={link.originalUrl} target="_blank" rel="noreferrer" title={link.originalUrl}>
          {link.originalUrl}
        </a>
      </div>
      <div className="link-meta">
        <span className="clicks" title="Переходов">
          {link.clicks} {plural(link.clicks, ["переход", "перехода", "переходов"])}
        </span>
        <span className="date" title={`Изменена ${dateFormat.format(new Date(link.updatedAt))}`}>
          {dateFormat.format(new Date(link.createdAt))}
        </span>
      </div>
      <div className="actions">
        <button type="button" onClick={copy}>
          {copied ? "Скопировано" : "Копировать"}
        </button>
        <button type="button" onClick={startEdit} disabled={busy}>
          Изменить
        </button>
        <button type="button" className="danger" onClick={remove} disabled={busy}>
          Удалить
        </button>
      </div>
      {error && <p className="error" role="alert">{error}</p>}
    </li>
  );
}

function plural(n: number, forms: [string, string, string]): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
  return forms[2];
}
