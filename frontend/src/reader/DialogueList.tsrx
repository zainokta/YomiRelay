import { useEffect, useRef, useState } from "octane";
import type { Dialogue } from "../api/client";
import { DialogueLine } from "./DialogueLine.tsrx";

export function DialogueList(props: { items: Dialogue[]; jumpToken: number }) {
  const container = useRef<HTMLDivElement | null>(null);
  const [nearBottom, setNearBottom] = useState(true);

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
  useEffect(() => {
    if (props.jumpToken === 0) return;
    const element = container.current;
    if (!element) return;
    element.scrollTo({ top: element.scrollHeight, behavior: "auto" });
    setNearBottom(true);
  }, [props.jumpToken]);

  return <div class="reader-list-wrap">
    <div class="reader-list" ref={container} onScroll={updatePosition}>
      {props.items.map((dialogue, index) => <DialogueLine key={`${dialogue.timestamp}-${index}`} dialogue={dialogue} />)}
    </div>
    {!nearBottom && <button class="jump" onClick={() => { const element = container.current; if (element) element.scrollTo({ top: element.scrollHeight, behavior: "auto" }); setNearBottom(true); }}>Jump to latest</button>}
  </div>;
}
