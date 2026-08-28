import { useState } from "octane";
import { createDialogueStream } from "./api/events";
import { GamesPage } from "./games/GamesPage.tsrx";
import { ReaderPage } from "./reader/ReaderPage.tsrx";

export function App() {
  const [section, setSection] = useState<"games" | "reader">("games");
  const [selectedGameId, setSelectedGameId] = useState<string>("");
  const [stream] = useState(createDialogueStream);

  const openReader = (gameId: string) => {
    setSelectedGameId(gameId);
    setSection("reader");
  };
  const selectReaderGame = (gameId: string) => setSelectedGameId(gameId);

  return <main class="app-shell">
    <header class="app-header">
      <div>
        <p class="eyebrow">LOCAL REN'PY DIALOGUE</p>
        <h1>YomiRelay</h1>
      </div>
      <nav aria-label="Main navigation">
        <button class={section === "games" ? "selected" : ""} onClick={() => setSection("games")}>Games</button>
        <button class={section === "reader" ? "selected" : ""} onClick={() => setSection("reader")}>Reader</button>
      </nav>
    </header>
    {section === "games"
      ? <GamesPage openReader={openReader} />
      : <ReaderPage gameId={selectedGameId} stream={stream} onSelectGame={selectReaderGame} />}
  </main>;
}
