import type { ComponentChildren } from 'preact';

// A small, deliberately non-HTML renderer: agent text can contain arbitrary
// markup. Keeping it as JSX text prevents it from executing inside the app.
function inline(text: string): ComponentChildren[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\(https?:\/\/[^\s)]+\))/g).map((part, i) => {
    if (part.startsWith('**')) return <strong key={i}>{part.slice(2, -2)}</strong>;
    if (part.startsWith('`')) return <code key={i}>{part.slice(1, -1)}</code>;
    const link = part.match(/^\[([^\]]+)\]\((https?:\/\/[^)]+)\)$/);
    if (link) return <a key={i} href={link[2]} target="_blank" rel="noreferrer">{link[1]}</a>;
    return part;
  });
}
export function RichText({ text, class: className = '' }: { text: string; class?: string }) {
  const lines = text.replace(/\r/g, '').split('\n');
  const blocks: ComponentChildren[] = [];
  let paragraph: string[] = [];
  const flush = () => {
    if (paragraph.length) { blocks.push(<p key={blocks.length}>{inline(paragraph.join('\n'))}</p>); paragraph = []; }
  };
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line.startsWith('```')) {
      flush(); const code: string[] = [];
      for (i++; i < lines.length && !lines[i].startsWith('```'); i++) code.push(lines[i]);
      blocks.push(<pre key={blocks.length}><code>{code.join('\n')}</code></pre>);
    } else if (/^#{1,4} /.test(line)) {
      flush(); const label = line.replace(/^#+ /, '');
      blocks.push(line.startsWith('# ') ? <h2 key={blocks.length}>{inline(label)}</h2> : <h3 key={blocks.length}>{inline(label)}</h3>);
    } else if (/^\s*[-*] /.test(line)) {
      flush(); const items = [line.replace(/^\s*[-*] /, '')];
      while (i + 1 < lines.length && /^\s*[-*] /.test(lines[i + 1])) items.push(lines[++i].replace(/^\s*[-*] /, ''));
      blocks.push(<ul key={blocks.length}>{items.map((item, j) => <li key={j}>{inline(item)}</li>)}</ul>);
    } else if (!line.trim() || /^---+$/.test(line)) { flush(); }
    else paragraph.push(line);
  }
  flush();
  return <div class={`rich-text ${className}`}>{blocks}</div>;
}
