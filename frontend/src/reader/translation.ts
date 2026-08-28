import type { Dialogue, Translation } from "../api/client";

export type TranslationMap = Readonly<Record<string, Translation>>;

export function translationKey(dialogue: Pick<Dialogue, "gameId" | "timestamp" | "text">): string {
	return `${dialogue.gameId}\u0000${dialogue.timestamp}\u0000${dialogue.text}`;
}
