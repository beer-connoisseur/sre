import { useCallback, useEffect, useState } from "react";
import { api, ApiError, Link } from "./api";
import { CreateLinkForm } from "./components/CreateLinkForm";
import { LinkRow } from "./components/LinkRow";

const PAGE_SIZE = 10;

export function App() {
  const [links, setLinks] = useState<Link[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (nextOffset: number) => {
    setLoading(true);
    setError(null);
    try {
      const page = await api.list(PAGE_SIZE, nextOffset);
      if (page.items.length === 0 && nextOffset > 0) {
        return load(Math.max(0, nextOffset - PAGE_SIZE));
      }
      setLinks(page.items);
      setTotal(page.total);
      setOffset(page.offset);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(0);
  }, [load]);

  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <main className="container">
      <header>
        <h1>URL Shortener</h1>
        <p className="subtitle">Сокращайте ссылки, меняйте адрес назначения и смотрите, сколько раз по ним перешли.</p>
      </header>

      <CreateLinkForm onCreated={() => void load(0)} />

      <section>
        <div className="list-header">
          <h2>
            Ссылки <span className="count">{total}</span>
          </h2>
          <button type="button" onClick={() => void load(offset)} disabled={loading}>
            Обновить
          </button>
        </div>

        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}

        {!error && !loading && links.length === 0 && <p className="empty">Пока нет ни одной ссылки.</p>}

        <ul className="links" aria-busy={loading}>
          {links.map((link) => (
            <LinkRow
              key={link.id}
              link={link}
              onChanged={(updated) => setLinks((prev) => prev.map((l) => (l.id === updated.id ? updated : l)))}
              onDeleted={() => void load(offset)}
            />
          ))}
        </ul>

        {pages > 1 && (
          <nav className="pager">
            <button type="button" onClick={() => void load(offset - PAGE_SIZE)} disabled={loading || offset === 0}>
              ← Назад
            </button>
            <span>
              {page} / {pages}
            </span>
            <button
              type="button"
              onClick={() => void load(offset + PAGE_SIZE)}
              disabled={loading || offset + PAGE_SIZE >= total}
            >
              Вперёд →
            </button>
          </nav>
        )}
      </section>
    </main>
  );
}
