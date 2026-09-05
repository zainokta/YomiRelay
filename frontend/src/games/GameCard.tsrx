import { useState } from "octane";
import { installHook, removeHook, type Game } from "../api/client";

export function GameCard(props: { game: Game; openReader: (gameId: string) => void; onChanged: () => Promise<void> }) {
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const changeHook = async (install: boolean) => {
    setBusy(true);
    setError("");
    setNotice("");
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
    } finally {
      setBusy(false);
    }
  };

  const managedRenPyHook = props.game.sourceStatus === "available";
  const sourceLabel = props.game.sourceStatus === "native-auto"
    ? "Live native hook (automatic)"
    : props.game.sourceStatus.startsWith("unsupported")
      ? "Unavailable"
      : props.game.hookInstalled
        ? "Hook installed"
        : "Hook not installed";

  return <article class="game-card">
    <div class="card-heading">
      <div><h3>{props.game.name}</h3><span class="app-id">Steam app {props.game.appId}</span></div>
      <span class={props.game.active ? "badge active" : "badge"}>{props.game.active ? "Active" : "Inactive"}</span>
    </div>
    <dl class="game-details">
      <div><dt>Engine</dt><dd>{props.game.engine === "nexas" ? "NeXAS" : props.game.engine}</dd></div>
      <div><dt>Source</dt><dd>{sourceLabel}</dd></div>
      <div><dt>Install path</dt><dd class="path">{props.game.installPath}</dd></div>
      {props.game.lastSeen && <div><dt>Last seen</dt><dd>{new Date(props.game.lastSeen).toLocaleString()}</dd></div>}
    </dl>
    {props.game.sourceMessage && <p class="status">{props.game.sourceMessage}</p>}
    {notice && <p class="notice" role="status">{notice}</p>}
    {error && <p class="error" role="alert">{error}</p>}
    <div class="card-actions">
      {managedRenPyHook && <button class="primary" onClick={() => void changeHook(true)} disabled={busy || props.game.hookInstalled}>Install Hook</button>}
      {managedRenPyHook && <button onClick={() => void changeHook(false)} disabled={busy || !props.game.hookInstalled}>Remove Hook</button>}
      <button onClick={() => props.openReader(props.game.appId)}>Open Reader</button>
    </div>
  </article>;
}
