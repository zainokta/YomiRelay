export type Game = {
  appId: string;
  name: string;
  installPath: string;
  engine: string;
  engineConfidence: string;
  dialogueSource: string;
  sourceStatus: string;
  sourceMessage?: string;
  executableHash?: string;
  hookInstalled: boolean;
  active: boolean;
  lastSeen?: string;
};

export type Dialogue = {
  engine?: string;
  gameId: string;
  gameName: string;
  speaker?: string;
  text: string;
  timestamp: string;
};

export type TranslationSegment = {
  text: string;
  kana: string;
  meaning: string;
};

export type Translation = {
  translation: string;
  segments: TranslationSegment[];
};

export type SourcePreviewCandidate = {
  address: string;
  speaker: string;
  text: string;
};

export type SourcePreview = {
  status: string;
  message: string;
  pid?: number;
  build: {
    architecture: string;
    sha256: string;
    verifiedBuild: boolean;
  };
  bytesRead: number;
  candidates: SourcePreviewCandidate[];
};

type ErrorEnvelope = { error?: { code?: string; message?: string } };

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({} as ErrorEnvelope)) as ErrorEnvelope;
    throw new Error(body.error?.message ?? `Request failed (${response.status})`);
  }
  if (response.status === 204) return undefined as T;
  return await response.json() as T;
}

export function listGames(refresh = false): Promise<Game[]> {
  return request<Game[]>(refresh ? "/api/games?refresh=1" : "/api/games");
}

export function installHook(appID: string): Promise<void> {
  return request<void>(`/api/games/${encodeURIComponent(appID)}/hook`, { method: "POST" });
}

export function removeHook(appID: string): Promise<void> {
  return request<void>(`/api/games/${encodeURIComponent(appID)}/hook`, { method: "DELETE" });
}

export function listDialogues(gameID: string): Promise<Dialogue[]> {
  return request<Dialogue[]>(`/api/dialogues?gameId=${encodeURIComponent(gameID)}`);
}

export function clearDialogues(gameID: string): Promise<void> {
  return request<void>(`/api/dialogues?gameId=${encodeURIComponent(gameID)}`, { method: "DELETE" });
}

export function translateDialogue(gameID: string, text: string, signal?: AbortSignal): Promise<Translation> {
  return request<Translation>("/api/translate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ gameId: gameID, text }),
    signal,
  });
}

export function getSourcePreview(appID: string): Promise<SourcePreview> {
  return request<SourcePreview>(`/api/games/${encodeURIComponent(appID)}/source-preview`);
}
