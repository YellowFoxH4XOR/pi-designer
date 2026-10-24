import { useState } from 'preact/hooks';
import type { Critique, CritiqueDecision, CritiquePanelist, CritiqueRole } from '../api';
import { Icon, type IconName } from './Icon';

const roleLabels: Record<CritiqueRole, string> = {
  designer: 'Designer intent', critic: 'Critic', brand: 'Brand', a11y: 'Accessibility', copy: 'Copy',
};
const decisions: Record<CritiqueDecision, { label: string; icon: IconName }> = {
  revise: { label: 'Revising', icon: 'refresh' },
  ship: { label: 'Ready to ship', icon: 'check' },
  ship_with_issues: { label: 'Shipped with open issues', icon: 'clock' },
};
const NOTE_LIMIT = 180;

/** The review panel's latest round, with earlier rounds kept as a score line. */
export function CritiqueCard({ critiques, live = false }: { critiques: Critique[]; live?: boolean }) {
  const [expanded, setExpanded] = useState('');
  const rounds = [...critiques].sort((a, b) => a.round - b.round);
  const latest = rounds[rounds.length - 1];
  if (!latest) return null;
  const decision = decisions[latest.decision] ?? { label: latest.decision, icon: 'eye' as const };
  const fixes: { item: string; panelist: CritiquePanelist }[] = [];
  const seen = new Set<string>();
  for (const panelist of latest.panelists ?? []) {
    for (const item of panelist.mustFix ?? []) {
      const key = item.trim().toLowerCase();
      if (!key || seen.has(key)) continue;
      seen.add(key);
      fixes.push({ item: item.trim(), panelist });
    }
  }
  const history = rounds.map((round) => `round ${round.round} scored ${round.composite.toFixed(1)}`).join(', ');
  return <section class={`critique-card ${live ? 'critique-card-live' : ''}`} aria-label="Design critique">
    <header class="critique-head">
      <h3>Critique · round {latest.round}{live && <span class="sr-only"> (in progress)</span>}</h3>
      <span class={`critique-decision is-${latest.decision}`}><Icon name={decision.icon} size={11}/>{decision.label}</span>
    </header>
    <div class="critique-overview">
      <p class="critique-score"><strong>{latest.composite.toFixed(1)}</strong><span>/ 10</span></p>
      {rounds.length > 1 && <p class="critique-history">
        <span class="critique-history-label">Rounds</span>
        <span aria-hidden="true">{rounds.map((round) => round.composite.toFixed(1)).join(' → ')}</span>
        <span class="sr-only">{`Score history: ${history}`}</span>
      </p>}
    </div>
    {latest.jury && <div class="critique-jury" aria-label="Independent jury">
      <div class="critique-jury-head"><h4>Independent jury</h4><span>{latest.jury.model}{typeof latest.selfComposite === 'number' ? ` · pi's own panel ${latest.selfComposite.toFixed(1)}` : ''}</span></div>
      <dl class="critique-jury-scores">{(['design', 'usability', 'creativity', 'content'] as const).map((k) => <div key={k}><dt>{k}</dt><dd>{latest.jury!.scores[k].toFixed(1)}</dd></div>)}</dl>
      {latest.jury.verdict && <p class="critique-notes">{latest.jury.verdict}</p>}
      {latest.jury.issues.length > 0 && <ul class="critique-jury-issues">{latest.jury.issues.map((issue, i) => <li key={i}>{issue}</li>)}</ul>}
    </div>}
    <ul class="critique-panel">
      {(latest.panelists ?? []).map((panelist, index) => {
        const notes = panelist.notes || '';
        const notesId = `critique-${latest.round}-${panelist.role}-${index}`;
        const open = expanded === notesId;
        const truncated = notes.length > NOTE_LIMIT;
        return <li key={notesId} class="critique-panelist">
          <div class="critique-panelist-head">
            <h4 class="critique-role">{roleLabels[panelist.role] || panelist.role}</h4>
            {typeof panelist.score === 'number' && <span class="critique-panelist-score">{panelist.score.toFixed(1)}<span class="sr-only"> out of 10</span></span>}
          </div>
          {notes && <p class="critique-notes">
            <span id={notesId}>{truncated && !open ? `${notes.slice(0, NOTE_LIMIT).trimEnd()}…` : notes}</span>
            {truncated && <button type="button" class="critique-more" aria-expanded={open} aria-controls={notesId} onClick={() => setExpanded(open ? '' : notesId)}>{open ? 'Less' : 'More'}</button>}
          </p>}
        </li>;
      })}
    </ul>
    {fixes.length > 0 && <div class="critique-fixes">
      <h4>{latest.decision === 'ship_with_issues' ? 'Open issues' : 'Must fix'}</h4>
      <ul>
        {fixes.map((fix, index) => <li key={`${index}-${fix.item}`}>
          <span class={`critique-fix-mark ${latest.decision === 'ship_with_issues' ? 'is-open' : ''}`} aria-hidden="true"/>
          <span class="critique-fix-text">{fix.item}</span>
          <span class="critique-fix-source">{roleLabels[fix.panelist.role] || fix.panelist.role}</span>
        </li>)}
      </ul>
    </div>}
  </section>;
}
