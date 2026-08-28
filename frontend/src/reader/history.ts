import type { Dialogue } from "../api/client.ts";

export function mergeDialogueHistory(history: readonly Dialogue[], buffered: readonly Dialogue[]): Dialogue[] {
  return [...history, ...buffered];
}
