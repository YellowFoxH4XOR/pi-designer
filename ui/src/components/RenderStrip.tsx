import type { Render } from '../api';
import { frames } from '../state';
import { Icon } from './Icon';

export function renderLabel(render: Render): string {
  if (render.viewport === 'web' || render.viewport === 'desktop' || render.viewport === 'mobile') {
    return frames[render.viewport].label;
  }
  const value = String(render.viewport || '');
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : 'Preview';
}

function renderNotes(render: Render): string {
  const errors = render.errors ?? 0;
  return [
    render.overflowX ? 'Overflows the viewport' : '',
    errors ? `${errors} ${errors === 1 ? 'error' : 'errors'}` : '',
  ].filter(Boolean).join(' · ');
}

/** Small, clickable render previews produced by render_preview. */
export function RenderStrip({ renders, onOpen }: { renders: Render[]; onOpen: (render: Render) => void }) {
  if (!renders.length) return null;
  return <section class="render-strip" aria-label="Preview renders">
    <div class="render-strip-head">
      <h3>Renders</h3>
      <span>{renders.length}</span>
    </div>
    <div class="render-list">
      {renders.map((render, index) => {
        const notes = renderNotes(render);
        return <button key={`${render.image}-${index}`} class={`render-thumb ${notes ? 'has-issue' : ''}`} onClick={() => onOpen(render)} aria-label={`Open ${renderLabel(render)} render at full size`}>
          <span class="render-frame"><img src={render.image} alt={`${renderLabel(render)} preview of the design`} loading="lazy"/></span>
          <span class="render-meta">
            <span class="render-viewport">{renderLabel(render)}</span>
            {notes && <span class="render-badge" title={notes}>{render.overflowX ? 'Overflow' : `${render.errors} ${render.errors === 1 ? 'error' : 'errors'}`}</span>}
          </span>
        </button>;
      })}
    </div>
  </section>;
}

export function RenderDialogBody({ render }: { render: Render }) {
  const notes = renderNotes(render);
  return <>
    <img class="render-full" src={render.image} alt={`${renderLabel(render)} render at full size`}/>
    <p class="render-dialog-meta"><Icon name="eye" size={14}/>{renderLabel(render)} · {render.path || 'index.html'}{notes ? ` · ${notes}` : ' · No layout issues detected'}</p>
  </>;
}
