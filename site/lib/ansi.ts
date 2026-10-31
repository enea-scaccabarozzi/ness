import type { CSSProperties } from 'react';

export type Seg = { text: string; style: CSSProperties };

/**
 * Parse terminal output with SGR codes (bold, dim, 24-bit colours) into styled segments per line.
 * The captures use only these; any other escape sequence is dropped.
 */
export function parseAnsi(src: string): Seg[][] {
  const lines: Seg[][] = [];
  for (const raw of src.replace(/\n$/, '').split('\n')) {
    const line: Seg[] = [];
    let st: CSSProperties = {};
    for (const part of raw.split(/(\x1b\[[0-9;:?]*[A-Za-z])/)) {
      if (!part) continue;
      const m = /^\x1b\[([0-9;]*)m$/.exec(part);
      if (m) st = apply(st, m[1]);
      else if (!part.startsWith('\x1b')) line.push({ text: part, style: st });
    }
    lines.push(line);
  }
  return lines;
}

function apply(prev: CSSProperties, codes: string): CSSProperties {
  const p = codes === '' ? [0] : codes.split(';').map(Number);
  let st = { ...prev };
  for (let i = 0; i < p.length; i++) {
    const c = p[i];
    if (c === 0) st = {};
    else if (c === 1) st.fontWeight = 700;
    else if (c === 2) st.opacity = 0.6;
    else if (c === 22) {
      delete st.fontWeight;
      delete st.opacity;
    } else if ((c === 38 || c === 48) && p[i + 1] === 2) {
      const rgb = `rgb(${p[i + 2]} ${p[i + 3]} ${p[i + 4]})`;
      if (c === 38) st.color = rgb;
      else st.backgroundColor = rgb;
      i += 4;
    } else if (c === 39) delete st.color;
    else if (c === 49) delete st.backgroundColor;
  }
  return st;
}

/** Visible width of the widest line, in columns. */
export function columns(lines: Seg[][]): number {
  return Math.max(1, ...lines.map((l) => [...l.map((s) => s.text).join('')].length));
}

export type Line = { segs: Seg[]; bg?: string; indent: number };

const plain = (l: Seg[]) => l.map((s) => s.text).join('');

/** Drop trailing blanks; a padded background (Claude Code's prompt bar) becomes the line's. */
function trimEnd(l: Seg[]): { segs: Seg[]; bg?: string } {
  const segs = [...l];
  let bg: string | undefined;
  while (segs.length && !segs.at(-1)!.text.trim()) bg ??= segs.pop()!.style.backgroundColor as string | undefined;
  if (segs.length) segs[segs.length - 1] = { ...segs.at(-1)!, text: segs.at(-1)!.text.trimEnd() };
  return { segs, bg };
}

function trimStart(l: Seg[]): Seg[] {
  const segs = [...l];
  while (segs.length && !segs[0].text.trim()) segs.shift();
  if (segs.length) segs[0] = { ...segs[0], text: segs[0].text.trimStart() };
  return segs;
}

const marker = /^\s*(?:[├└]─\s+)?(?:(?:[-●○❯✻⎿]|\d+\.|\d+ [+ ])\s*)?/;

/**
 * Undo the terminal's hard wraps so the browser can wrap the text to any width: a line that
 * starts indented, right after a line that ran to the terminal's edge, continues it. Each line
 * gets a hanging indent to where its text starts, as in the terminal. Diffs pass join=false:
 * their wrapped lines carry a marker and can break mid-word.
 */
export function reflow(lines: Seg[][], cols: number, join = true): Line[] {
  const out: { segs: Seg[]; bg?: string }[] = [];
  let prevLen = 0;
  for (const raw of lines) {
    const t = plain(raw);
    const cur = trimEnd(raw);
    const prev = out.at(-1);
    if (join && prev && prev.segs.length && prevLen >= cols - 30 && /^\s+(?!\d+\.\s)[^\s\-●○❯✻⎿+]/.test(t)) {
      prev.segs = [...prev.segs, { text: ' ', style: {} }, ...trimStart(cur.segs)];
      prev.bg ??= cur.bg;
    } else out.push(cur);
    prevLen = [...t.trimEnd()].length;
  }
  return out.map((l) => ({ ...l, indent: Math.min(12, [...(marker.exec(plain(l.segs))?.[0] ?? '')].length) }));
}
