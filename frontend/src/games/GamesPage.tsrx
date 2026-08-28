import { useEffect, useState } from "octane";
import { listGames, type Game } from "../api/client";
import { GameCard } from "./GameCard.tsrx";

export function GamesPage(props: { openReader: (gameId: string) => void }) {
  const [games, setGames] = useState<Game[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const load = async (refresh: boolean) => {
    setLoading(true);
    setError("");
    try {
      setGames(await listGames(refresh));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not load games.");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { void load(false); }, []);

  return <section class="panel" aria-labelledby="games-heading">
    <div class="section-heading">
      <div><p class="eyebrow">DISCOVERY</p><h2 id="games-heading">Installed Ren'Py games</h2></div>
      <button class="primary" onClick={() => void load(true)} disabled={loading}>Refresh</button>
    </div>
    {error && <p class="error" role="alert">{error}</p>}
    {loading && games.length === 0 && <p class="status">Looking for Steam libraries…</p>}
    {!loading && games.length === 0 && <p class="status">No Ren'Py games were detected.</p>}
    <ul class="game-list">
      {games.map((game) => <li key={game.appId}><GameCard game={game} openReader={props.openReader} onChanged={() => load(false)} /></li>)}
    </ul>
  </section>;
}
