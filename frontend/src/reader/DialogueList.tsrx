import { useEffect, useRef, useState } from "octane";
import type { Dialogue } from "../api/client";
import { DialogueLine, type GlossHandler } from "./DialogueLine.tsrx";
import { translationKey, type TranslationMap } from "./translation";

type Gloss = { kana: string; meaning: string; left: number; top: number; above: boolean };

export function DialogueList(props: { items: Dialogue[]; translations: TranslationMap }) {
  const container = useRef<HTMLDivElement | null>(null);
  const [nearBottom, setNearBottom] = useState(true);
  const [gloss, setGloss] = useState<Gloss | null>(null);

  const updatePosition = () => {
    const element = container.current;
    if (!element) return;
    setNearBottom(element.scrollHeight - element.scrollTop - element.clientHeight <= 72);
  };
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    if (nearBottom) element.scrollTo({ top: element.scrollHeight, behavior: "auto" });
  }, [props.items.length]);
  const showGloss: GlossHandler = (target, kana, meaning) => {
    const rect = target.getBoundingClientRect();
    const halfWidth = Math.min(192, window.innerWidth * 0.35);
    const left = Math.min(Math.max(rect.left + rect.width / 2, halfWidth + 12), window.innerWidth - halfWidth - 12);
    const above = rect.top > 96;
    setGloss({ kana, meaning, left, top: above ? rect.top - 8 : rect.bottom + 8, above });
  };
  const hideGloss = () => setGloss(null);
  return <div class="reader-list-wrap">
    <div class="reader-list" ref={container} onScroll={() => { updatePosition(); hideGloss(); }}>
      {props.items.map((dialogue, index) => <DialogueLine
        key={`${dialogue.timestamp}-${index}`}
        dialogue={dialogue}
        translation={props.translations[translationKey(dialogue)]}
        onGloss={showGloss}
        onHideGloss={hideGloss}
      />)}
    </div>
    {!nearBottom && <button class="jump" onClick={() => { const element = container.current; if (element) element.scrollTo({ top: element.scrollHeight, behavior: "auto" }); setNearBottom(true); }}>Jump to latest</button>}
    {gloss && <div
      id="reader-gloss-tooltip"
      class={`gloss-tooltip ${gloss.above ? "above" : "below"}`}
      style={`--gloss-left: ${gloss.left}px; --gloss-top: ${gloss.top}px;`}
      role="tooltip"
    >{gloss.kana}<span aria-hidden="true"> · </span>{gloss.meaning}</div>}
  </div>;
}
