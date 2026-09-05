import { useState } from "octane";
import { installHook, removeHook, inspectSource, publishSourceCandidate, type SourceSnapshot, type Game } from "../api/client";

export function GameCard(props: { game: Game; openReader: (gameId: string) => void; onChanged: () => Promise<void> }) {
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [snapshot, setSnapshot] = useState<SourceSnapshot | null>(null);

  const inspect = async () => {
    setBusy(true); setError(""); setSnapshot(null);
    try { setSnapshot(await inspectSource(props.game.appId)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Native inspection failed."); }
    finally { setBusy(false); }
  };

  const publish = async (raw: string) => {
    setBusy(true); setError(""); setNotice("");
    try {
      await publishSourceCandidate(props.game.appId, raw);
      setNotice("Candidate sent to Reader history.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not send candidate to Reader.");
    } finally { setBusy(false); }
  };

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
      <div><dt>Engine</dt><dd>{props.game.engine === "nexas" ? "NeXAS" : props.game.engine}</dd></div>
      <div><dt>Hook</dt><dd>{props.game.sourceStatus === "experimental" ? "Experimental — capture not ready" : props.game.sourceStatus.startsWith("unsupported") ? "Unavailable" : props.game.hookInstalled ? "Installed" : "Not installed"}</dd></div>
      <div><dt>Install path</dt><dd class="path">{props.game.installPath}</dd></div>
      {props.game.lastSeen && <div><dt>Last seen</dt><dd>{new Date(props.game.lastSeen).toLocaleString()}</dd></div>}
    </dl>
    {props.game.sourceMessage && <p class="status">{props.game.sourceMessage}</p>}
    {notice && <p class="notice" role="status">{notice}</p>}
    {error && <p class="error" role="alert">{error}</p>}
    <div class="card-actions">
      <button class="primary" onClick={() => void changeHook(true)} disabled={busy || props.game.hookInstalled || props.game.sourceStatus !== "available"}>Install Hook</button>
      <button onClick={() => void changeHook(false)} disabled={busy || !props.game.hookInstalled}>Remove Hook</button>
      <button onClick={() => props.openReader(props.game.appId)}>Open Reader</button>
      {props.game.sourceStatus === "experimental" && <button onClick={() => void inspect()} disabled={busy}>{busy ? "Inspecting…" : "Inspect native source"}</button>}
    </div>
    {snapshot && <section class="source-debug" aria-label="Native source diagnostics">
      <h4>Native source diagnostics</h4>
      <p role="status">{snapshot.message}</p>
      <p class="status">Process {snapshot.pid ?? "not found"} · {snapshot.build.architecture} · {snapshot.candidates.length} candidates</p>
      <details><summary>Executable SHA-256</summary><p class="path">{snapshot.build.sha256}</p></details>
      {snapshot.candidates.length === 0 && <p>No speaker-tagged text found. Open a story scene, then inspect again.</p>}
      <ul class="source-candidates">
        {snapshot.candidates.map((candidate) => <li key={candidate.address}><p lang="ja">{candidate.raw}</p><button onClick={() => void publish(candidate.raw)} disabled={busy}>Send to Reader</button></li>)}
      </ul>
      <p class="status">Diagnostic candidates only. These are not sent to Reader history.</p>
    </section>}
  </article>;
}
