import { FormEvent, useState } from "react";
import { api, ApiError, Link } from "../api";

interface Props {
  onCreated: (link: Link) => void;
}

export function CreateLinkForm({ onCreated }: Props) {
  const [url, setUrl] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const link = await api.create(url.trim(), code.trim());
      setUrl("");
      setCode("");
      onCreated(link);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="card create-form" onSubmit={submit}>
      <label className="field field-url">
        <span>Длинная ссылка</span>
        <input
          type="url"
          required
          placeholder="https://example.com/very/long/path"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
      </label>
      <label className="field field-code">
        <span>Свой код (необязательно)</span>
        <input
          type="text"
          placeholder="my-link"
          pattern="[A-Za-z0-9_\-]{3,32}"
          title="3–32 символа: латиница, цифры, _ или -"
          value={code}
          onChange={(e) => setCode(e.target.value)}
        />
      </label>
      <button type="submit" className="primary" disabled={busy}>
        {busy ? "Сокращаю…" : "Сократить"}
      </button>
      {error && <p className="error" role="alert">{error}</p>}
    </form>
  );
}
