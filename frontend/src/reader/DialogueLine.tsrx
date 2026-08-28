import type { Dialogue } from "../api/client";

export function DialogueLine(props: { dialogue: Dialogue }) {
  return <article class="dialogue">
    {props.dialogue.speaker && <div class="speaker">{props.dialogue.speaker}</div>}
    <div class="dialogue-text" lang="ja">{props.dialogue.text}</div>
    <time dateTime={props.dialogue.timestamp}>{new Date(props.dialogue.timestamp).toLocaleTimeString()}</time>
  </article>;
}
