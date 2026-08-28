import { mergeDialogueHistory } from "./history.ts";

function assertDeepEqual(actual: unknown, expected: unknown): void {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`Expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

const dialogue = (text: string) => ({
  gameId: "game-1",
  gameName: "Game One",
  text,
  timestamp: `2026-08-28T00:00:0${text.length}Z`,
});

assertDeepEqual(
  mergeDialogueHistory([dialogue("history")], [dialogue("live one"), dialogue("live two")]),
  [dialogue("history"), dialogue("live one"), dialogue("live two")],
);
assertDeepEqual(mergeDialogueHistory([], [dialogue("live")]), [dialogue("live")]);
