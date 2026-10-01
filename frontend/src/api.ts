export interface Link {
  id: string;
  code: string;
  originalUrl: string;
  clicks: number;
  createdAt: string;
  updatedAt: string;
}

export interface LinkPage {
  items: Link[];
  total: number;
  limit: number;
  offset: number;
}

export interface LinkUpdate {
  originalUrl?: string;
  code?: string;
}

const API_BASE = "/api/v1";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

const messages: Record<string, string> = {
  validation_error: "Проверьте данные",
  code_taken: "Такой код уже занят",
  not_found: "Ссылка не найдена",
  internal_error: "Ошибка сервера, попробуйте позже",
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(API_BASE + path, {
      ...init,
      headers: { "Content-Type": "application/json", ...init?.headers },
    });
  } catch {
    throw new ApiError(0, "network_error", "Сервер недоступен");
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const body = await response.json().catch(() => null);
  if (!response.ok) {
    const code: string = body?.error?.code ?? "internal_error";
    const detail: string | undefined = body?.error?.message;
    const text = messages[code] ?? "Неизвестная ошибка";
    throw new ApiError(response.status, code, code === "validation_error" && detail ? `${text}: ${detail}` : text);
  }

  return body as T;
}

export const api = {
  list: (limit: number, offset: number) => request<LinkPage>(`/links?limit=${limit}&offset=${offset}`),
  create: (originalUrl: string, code?: string) =>
    request<Link>("/links", { method: "POST", body: JSON.stringify({ originalUrl, code: code || undefined }) }),
  update: (id: string, update: LinkUpdate) =>
    request<Link>(`/links/${encodeURIComponent(id)}`, { method: "PATCH", body: JSON.stringify(update) }),
  remove: (id: string) => request<void>(`/links/${encodeURIComponent(id)}`, { method: "DELETE" }),
};

export function shortUrl(code: string): string {
  return `${window.location.origin}/r/${code}`;
}
