import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

// Подсветка markdown в духе книжной страницы: заголовки крупнее, а не подчёркнуты,
// служебные символы (#, *, >) приглушены
const markdownStyle = HighlightStyle.define([
  { tag: t.heading1, fontSize: "1.55em", fontWeight: "650", lineHeight: "1.3" },
  { tag: t.heading2, fontSize: "1.3em", fontWeight: "650" },
  { tag: [t.heading3, t.heading4, t.heading5, t.heading6], fontSize: "1.1em", fontWeight: "650" },
  { tag: t.strong, fontWeight: "700" },
  { tag: t.emphasis, fontStyle: "italic" },
  { tag: t.strikethrough, textDecoration: "line-through" },
  { tag: t.link, color: "var(--accent)", textDecoration: "underline" },
  { tag: t.url, color: "var(--ink-soft)" },
  { tag: t.quote, color: "var(--ink-soft)", fontStyle: "italic" },
  { tag: t.monospace, fontFamily: "ui-monospace, 'Cascadia Code', Menlo, monospace", fontSize: "0.9em" },
  { tag: [t.processingInstruction, t.meta], color: "var(--ink-soft)", opacity: "0.7" },
]);

export const markdownHighlighting = syntaxHighlighting(markdownStyle);
