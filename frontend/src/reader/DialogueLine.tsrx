import type { Dialogue, Translation } from "../api/client";

export type GlossHandler = (target: HTMLElement, kana: string, meaning: string) => void;

function JapaneseText(props: { dialogue: Dialogue; translation?: Translation; onGloss?: GlossHandler; onHideGloss?: () => void }) {
  if (!props.translation) {
    return <div class="dialogue-text" lang="ja">{props.dialogue.text}</div>;
  }
  return <div class="dialogue-text" lang="ja">
    {props.translation.segments.map((segment, index) => segment.kana && segment.meaning
      ? <span
          class="gloss-word"
          data-kana={segment.kana}
          data-meaning={segment.meaning}
          aria-label={`${segment.text}: ${segment.kana}; ${segment.meaning}`}
          tabIndex={0}
          onMouseEnter={(event) => props.onGloss?.(event.currentTarget, segment.kana, segment.meaning)}
          onMouseLeave={() => props.onHideGloss?.()}
          onFocus={(event) => props.onGloss?.(event.currentTarget, segment.kana, segment.meaning)}
          onBlur={() => props.onHideGloss?.()}
          key={`${index}-${segment.text}`}
        >{segment.text}</span>
      : segment.text)}
  </div>;
}

export function DialogueLine(props: { dialogue: Dialogue; translation?: Translation; onGloss?: GlossHandler; onHideGloss?: () => void }) {
  return <article class="dialogue">
    {props.dialogue.speaker && <div class="speaker">{props.dialogue.speaker}</div>}
    <JapaneseText dialogue={props.dialogue} translation={props.translation} onGloss={props.onGloss} onHideGloss={props.onHideGloss} />
    {props.translation && <div class="dialogue-translation" lang="en">{props.translation.translation}</div>}
    <time dateTime={props.dialogue.timestamp}>{new Date(props.dialogue.timestamp).toLocaleTimeString()}</time>
  </article>;
}
