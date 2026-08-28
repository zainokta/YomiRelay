import { useState } from "octane";
import { installHook, removeHook, type Game } from "../api/client";

export function GameCard(props: { game: Game; openReader: (gameId: string) => void }) {
  const [game, setGame] = useState(props.game);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const changeHook = async (install: boolean) => {
    setBusy(true); setError(""); setNotice("");
    try {
      if (install) {
        await installHook(game.appId);
        setGame({ ...game, hookInstalled: true });
        setNotice("Restart the game to activate the hook.");
      } else {
        await removeHook(game.appId);
        setGame({ ...game, hookInstalled: false });
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Hook action failed.");
    } finally { setBusy(false); }
  };

  return <article class="game-card">
    <div class="card-heading"><div><h3>{game.name}</h3><span class="app-id">Steam app {game.appId}</span></div><span class={game.active ? "badge active" : "badge"}>{game.active ? "Active" : "Inactive"}</span></div>
    <dl class="game-details">
      <div><dt>Engine</dt><dd>{game.engine}</dd></div>
      <div><dt>Hook</dt><dd>{game.hookInstalled ? "Installed" : "Not installed"}</dd></div>
      <div><dt>Install path</dt><dd class="path">{game.installPath}</dd></div>
      {game.lastSeen && <div><dt>Last seen</dt><dd>{new Date(game.lastSeen).toLocaleString()}</dd></div>}
    </dl>
    {notice && <p class="notice" role="status">{notice}</p>}
    {error && <p class="error" role="alert">{error}</p>}
    <div class="card-actions">
      <button class="primary" onClick={() => void changeHook(true)} disabled={busy || game.hookInstalled}>Install Hook</button>
      <button onClick={() => void changeHook(false)} disabled={busy || !game.hookInstalled}>Remove Hook</button>
      <button onClick={() => props.openReader(game.appId)}>Open Reader</button>
    </div>
  </article>;
}
