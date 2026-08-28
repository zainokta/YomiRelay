import { useState } from "octane";
import { installHook, removeHook, type Game } from "../api/client";

export function GameCard(props: { game: Game; openReader: (gameId: string) => void; onChanged: () => Promise<void> }) {
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const changeHook = async (install: boolean) => {
    setBusy(true); setError(""); setNotice("");
    try {
      if (install) {
        await installHook(props.game.appId);
        setNotice("Restart the game to activate the hook.");
      } else {
        await removeHook(props.game.appId);
      }
      await props.onChanged();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Hook action failed.");
    } finally { setBusy(false); }
  };

  return <article class="game-card">
    <div class="card-heading"><div><h3>{props.game.name}</h3><span class="app-id">Steam app {props.game.appId}</span></div><span class={props.game.active ? "badge active" : "badge"}>{props.game.active ? "Active" : "Inactive"}</span></div>
    <dl class="game-details">
      <div><dt>Engine</dt><dd>{props.game.engine}</dd></div>
      <div><dt>Hook</dt><dd>{props.game.hookInstalled ? "Installed" : "Not installed"}</dd></div>
      <div><dt>Install path</dt><dd class="path">{props.game.installPath}</dd></div>
      {props.game.lastSeen && <div><dt>Last seen</dt><dd>{new Date(props.game.lastSeen).toLocaleString()}</dd></div>}
    </dl>
    {notice && <p class="notice" role="status">{notice}</p>}
    {error && <p class="error" role="alert">{error}</p>}
    <div class="card-actions">
      <button class="primary" onClick={() => void changeHook(true)} disabled={busy || props.game.hookInstalled}>Install Hook</button>
      <button onClick={() => void changeHook(false)} disabled={busy || !props.game.hookInstalled}>Remove Hook</button>
      <button onClick={() => props.openReader(props.game.appId)}>Open Reader</button>
    </div>
  </article>;
}
