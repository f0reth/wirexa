import { EditorView } from "@codemirror/view";

// URL-encoded down-chevron SVG. Used as a CSS mask: its alpha cuts out the
// shape and the visible color comes from `background-color: currentColor`.
// The span sets no `color`, so it inherits the gutter's color — making the
// icon match the line numbers and adapt to light/dark themes automatically.
const chevronDown =
  "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cpath d='M4 6.5 8 10 12 6.5' fill='none' stroke='black' stroke-width='1.6' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E\")";

/**
 * Shared gutter styling for the CodeMirror editors that use `basicSetup`
 * (which bundles line numbers + a fold gutter).
 *
 * The default fold marker is a text glyph (⌄/›) whose font metrics sit
 * off-center, so it never aligns with the line numbers. We replace it with a
 * crisp SVG icon (geometric center, font-independent) and size it up.
 *
 * Lives in an `EditorView.theme()` (instance-scoped, high specificity) rather
 * than global CSS so it reliably overrides CodeMirror's runtime-injected base
 * styles.
 */
export const gutterTheme = EditorView.theme({
  // Line numbers: 10px, vertically centered and right-aligned.
  ".cm-gutters": { fontSize: "0.625rem" },
  ".cm-lineNumbers .cm-gutterElement": {
    display: "flex",
    alignItems: "center",
    justifyContent: "flex-end",
  },
  // Fold gutter: center the marker within the line.
  ".cm-foldGutter .cm-gutterElement": {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    padding: "0",
  },
  // Replace the ⌄/› text glyph with an SVG icon, larger than the line numbers.
  // No `color` is set, so the icon inherits the gutter color (= line numbers).
  // Hidden by default; revealed only while the gutter (sidebar) is hovered.
  ".cm-foldGutter .cm-gutterElement > span": {
    display: "block",
    width: "0.875rem",
    height: "0.875rem",
    fontSize: "0", // hide the original glyph character
    backgroundColor: "currentColor",
    mask: `${chevronDown} center / contain no-repeat`,
    WebkitMask: `${chevronDown} center / contain no-repeat`,
    opacity: "0",
    transition: "opacity var(--transition-fast)",
  },
  // Reveal all fold chevrons when the mouse is over the gutter sidebar.
  ".cm-gutters:hover .cm-foldGutter .cm-gutterElement > span": {
    opacity: "1",
  },
  // When a line is folded (closed), point the chevron right and keep it visible
  // even when the gutter isn't hovered, so collapsed regions stay obvious.
  // open/closed can only be distinguished via the marker's title (no state
  // class is exposed); the app has no i18n, so the English "Unfold line" holds.
  ".cm-foldGutter .cm-gutterElement > span[title='Unfold line']": {
    transform: "rotate(-90deg)",
    opacity: "1",
  },
});
