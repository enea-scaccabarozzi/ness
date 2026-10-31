import type { Blockquote, Paragraph, Root, RootContent } from 'mdast';

// GitHub alert -> Fumadocs <Callout type>. Same Markdown renders on github.com and here.
const types: Record<string, string> = {
  NOTE: 'info',
  TIP: 'success',
  IMPORTANT: 'idea',
  WARNING: 'warning',
  CAUTION: 'error',
};
const marker = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*\n?/;

function convert(node: Blockquote): RootContent | undefined {
  const first = node.children[0] as Paragraph | undefined;
  const text = first?.type === 'paragraph' ? first.children[0] : undefined;
  if (text?.type !== 'text') return;
  const m = marker.exec(text.value);
  if (!m) return;
  text.value = text.value.slice(m[0].length);
  if (text.value === '') first!.children.shift();
  if (first!.children.length === 0) node.children.shift();
  // mdxJsxFlowElement is passed through by the MDX compiler even for plain .md files.
  return {
    type: 'mdxJsxFlowElement',
    name: 'Callout',
    attributes: [{ type: 'mdxJsxAttribute', name: 'type', value: types[m[1]] }],
    children: node.children,
  } as unknown as RootContent;
}

function walk(parent: { children?: RootContent[] }) {
  if (!parent.children) return;
  parent.children = parent.children.map((child) => {
    if (child.type === 'blockquote') {
      const callout = convert(child);
      if (callout) {
        walk(callout as { children?: RootContent[] });
        return callout;
      }
    }
    walk(child as { children?: RootContent[] });
    return child;
  });
}

export function remarkGithubAlerts() {
  return (tree: Root) => walk(tree);
}
