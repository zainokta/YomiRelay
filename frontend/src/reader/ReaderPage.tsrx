import { useEffect, useState } from "octane";
import { clearDialogues, listDialogues, listGames, type Dialogue, type Game } from "../api/client";
import type { createDialogueStream } from "../api/events";
import { DialogueList } from "./DialogueList.tsrx";

type DialogueStream = ReturnType<typeof createDialogueStream>;

export function ReaderPage(props: { gameId: string; stream: DialogueStream; onSelectGame: (gameId: string) => void }) {
  const [games, setGames] = useState<Game[]>([]);
  const [items, setItems] = useState<Dialogue[]>([]);
  const [error, setError] = useState("");
  const [jumpToken, setJumpToken] = useState(0);

  useEffect(() => {
    let current = true;
    void listGames().then((value) => { if (current) setGames(value); }).catch((reason) => { if (current) setError(reason instanceof Error ? reason.message : "Could not load games."); });
    return () => { current = false; };
  }, []);
  useEffect(() => {
    if (!props.gameId) { setItems([]); return; }
    let current = true;
    setError("");
    void listDialogues(props.gameId).then((value) => { if (current) setItems(value); }).catch((reason) => { if (current) setError(reason instanceof Error ? reason.message : "Could not load dialogue."); });
    const unsubscribe = props.stream.subscribe((value) => {
      if (value.gameId === props.gameId) setItems((previous) => [...previous, value]);
    });
    return () => { current = false; unsubscribe(); };
  }, [props.gameId, props.stream]);

  const clear = async () => {
    if (!props.gameId) return;
    try { await clearDialogues(props.gameId); setItems([]); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Could not clear dialogue."); }
  };
  const selectedGame = games.find((game) => game.appId === props.gameId);

  return <section class="panel reader-panel" aria-labelledby="reader-heading">
    <div class="section-heading">
      <div><p class="eyebrow">READER</p><h2 id="reader-heading">{selectedGame?.name ?? "Select a game"}</h2></div>
      <div class="reader-controls">
        <label>Game <select value={props.gameId} onChange={(event) => props.onSelectGame(event.currentTarget.value)}><option value="">Choose a game</option>{games.map((game) => <option value={game.appId} key={game.appId}>{game.name}</option>)}</select></label>
        <button onClick={() => void clear()} disabled={!props.gameId}>Clear History</button>
      </div>
    </div>
    {error && <p class="error" role="alert">{error}</p>}
    {!props.gameId ? <p class="status">Choose a detected game to read its dialogue.</p> : <DialogueList items={items} jumpToken={jumpToken} />}
    <button class="hidden-jump" onClick={() => setJumpToken((value) => value + 1)} aria-label="Jump to latest">Jump to latest</button>
  </section>;
}
