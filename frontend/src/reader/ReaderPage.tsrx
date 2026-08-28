import { useEffect, useRef, useState } from "octane";
import { clearDialogues, listDialogues, listGames, translateDialogue, type Dialogue, type Game, type Translation } from "../api/client";
import type { createDialogueStream } from "../api/events";
import { DialogueList } from "./DialogueList.tsrx";
import { mergeDialogueHistory } from "./history";
import { translationKey, type TranslationMap } from "./translation";

type DialogueStream = ReturnType<typeof createDialogueStream>;

export function ReaderPage(props: { gameId: string; stream: DialogueStream; onSelectGame: (gameId: string) => void }) {
  const [games, setGames] = useState<Game[]>([]);
  const [items, setItems] = useState<Dialogue[]>([]);
  const [error, setError] = useState("");
  const [translationEnabled, setTranslationEnabled] = useState(false);
  const [translating, setTranslating] = useState(false);
  const [translations, setTranslations] = useState<TranslationMap>({});
  const translationsRef = useRef<TranslationMap>({});
  const queueRef = useRef<Dialogue[]>([]);
  const queuedKeysRef = useRef(new Set<string>());
  const activeKeyRef = useRef("");
  const processingRef = useRef(false);
  const enabledRef = useRef(false);
  const generationRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);

  const stopPending = () => {
    generationRef.current += 1;
    abortRef.current?.abort();
    abortRef.current = null;
    activeKeyRef.current = "";
    queueRef.current = [];
    queuedKeysRef.current.clear();
  };

  const disableTranslation = () => {
    enabledRef.current = false;
    stopPending();
    setTranslationEnabled(false);
    setTranslating(false);
  };

  const rememberTranslation = (key: string, result: Translation) => {
    const next = { ...translationsRef.current, [key]: result };
    translationsRef.current = next;
    setTranslations(next);
  };

  const processTranslationQueue = async () => {
    if (processingRef.current || !enabledRef.current) return;
    processingRef.current = true;
    setTranslating(true);
    const generation = generationRef.current;
    try {
      while (enabledRef.current && generation === generationRef.current && queueRef.current.length > 0) {
        const dialogue = queueRef.current.shift()!;
        const key = translationKey(dialogue);
        queuedKeysRef.current.delete(key);
        if (translationsRef.current[key]) continue;
        activeKeyRef.current = key;
        const controller = new AbortController();
        abortRef.current = controller;
        try {
          const result = await translateDialogue(dialogue.gameId, dialogue.text, controller.signal);
          if (!enabledRef.current || generation !== generationRef.current) break;
          rememberTranslation(key, result);
        } catch {
          if (enabledRef.current && generation === generationRef.current) disableTranslation();
          break;
        } finally {
          if (activeKeyRef.current === key) activeKeyRef.current = "";
          if (abortRef.current === controller) abortRef.current = null;
        }
      }
    } finally {
      processingRef.current = false;
      setTranslating(false);
    }
  };

  const enqueueTranslations = (values: readonly Dialogue[]) => {
    if (!enabledRef.current) return;
    for (const dialogue of values) {
      const key = translationKey(dialogue);
      if (translationsRef.current[key] || key === activeKeyRef.current || queuedKeysRef.current.has(key)) continue;
      queuedKeysRef.current.add(key);
      queueRef.current.push(dialogue);
    }
    void processTranslationQueue();
  };

  const enableTranslation = () => {
    enabledRef.current = true;
    setTranslationEnabled(true);
    enqueueTranslations(items);
  };

  useEffect(() => () => {
    enabledRef.current = false;
    stopPending();
  }, []);

  useEffect(() => {
    let current = true;
    void listGames().then((value) => { if (current) setGames(value); }).catch((reason) => { if (current) setError(reason instanceof Error ? reason.message : "Could not load games."); });
    return () => { current = false; };
  }, []);
  useEffect(() => {
    if (!props.gameId) { setItems([]); return; }
    let current = true;
    let historyLoaded = false;
    const buffered: Dialogue[] = [];
    setError("");
    const unsubscribe = props.stream.subscribe((value) => {
      if (value.gameId !== props.gameId) return;
      if (!historyLoaded) buffered.push(value);
      else setItems((previous) => [...previous, value]);
    });
    void listDialogues(props.gameId).then((value) => {
      if (!current) return;
      historyLoaded = true;
      setItems(mergeDialogueHistory(value, buffered));
    }).catch((reason) => {
      if (!current) return;
      historyLoaded = true;
      setItems(buffered);
      setError(reason instanceof Error ? reason.message : "Could not load dialogue.");
    });
    return () => { current = false; unsubscribe(); };
  }, [props.gameId, props.stream]);

  useEffect(() => {
    if (translationEnabled) enqueueTranslations(items);
  }, [items, props.gameId, translationEnabled]);

  const clear = async () => {
    if (!props.gameId) return;
    try {
      stopPending();
      translationsRef.current = {};
      setTranslations({});
      await clearDialogues(props.gameId);
      setItems([]);
    }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Could not clear dialogue."); }
  };
  const selectedGame = games.find((game) => game.appId === props.gameId);

  return <section class="panel reader-panel" aria-labelledby="reader-heading">
    <div class="section-heading">
      <div><p class="eyebrow">READER</p><h2 id="reader-heading">{selectedGame?.name ?? "Select a game"}</h2></div>
      <div class="reader-controls">
        <label>Game <select value={props.gameId} onChange={(event) => props.onSelectGame(event.currentTarget.value)}><option value="">Choose a game</option>{games.map((game) => <option value={game.appId} key={game.appId}>{game.name}</option>)}</select></label>
        <button onClick={() => void clear()} disabled={!props.gameId}>Clear History</button>
        <button class="translation-toggle" aria-pressed={translationEnabled} onClick={() => translationEnabled ? disableTranslation() : enableTranslation()}>
          {translationEnabled ? "Disable translation" : "Enable English translation"}
        </button>
        {translationEnabled && translating && <span class="translation-status" role="status" aria-live="polite">Translating…</span>}
      </div>
    </div>
    {error && <p class="error" role="alert">{error}</p>}
    {!props.gameId ? <p class="status">Choose a detected game to read its dialogue.</p> : <DialogueList items={items} translations={translationEnabled ? translations : {}} />}
  </section>;
}
