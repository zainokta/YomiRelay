import type { Dialogue } from "./client";

export type Unsubscribe = () => void;

export function createDialogueStream(): {
  subscribe(listener: (dialogue: Dialogue) => void): Unsubscribe;
  close(): void;
} {
  const listeners = new Set<(dialogue: Dialogue) => void>();
  let source: EventSource | undefined;
  const connect = () => {
    if (source) return;
    source = new EventSource("/api/events");
    source.addEventListener("dialogue", (event) => {
      try {
        const dialogue = JSON.parse((event as MessageEvent).data) as Dialogue;
        for (const listener of listeners) listener(dialogue);
      } catch (error) {
        console.warn("YomiRelay dialogue event could not be parsed", error);
      }
    });
    source.addEventListener("error", () => console.warn("YomiRelay dialogue stream disconnected"));
  };
  const close = () => {
    source?.close();
    source = undefined;
  };
  return {
    subscribe(listener) {
      listeners.add(listener);
      connect();
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) close();
      };
    },
    close() {
      listeners.clear();
      close();
    }
  };
}
