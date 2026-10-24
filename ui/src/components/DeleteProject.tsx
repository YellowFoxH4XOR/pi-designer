import { useState } from 'preact/hooks';
import type { Project } from '../api';
import { Dialog } from './common';
import { Icon } from './Icon';

// Deleting is permanent: the daemon removes the project's files, renders,
// pi sessions, conversations and run history together.
export function DeleteProjectDialog({ project, running, onClose, onDelete }: {
  project: Project; running: boolean; onClose: () => void; onDelete: (p: Project) => Promise<void>;
}) {
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState('');
  async function confirm() {
    setDeleting(true); setError('');
    try { await onDelete(project); }
    catch (e) { setError(String(e).replace(/^Error: /, '')); setDeleting(false); }
  }
  return <Dialog title="Delete project" onClose={() => { if (!deleting) onClose(); }}>
    <p class="dialog-intro">Delete <strong>{project.name}</strong>? Its files, previews, renders, conversations and run history are removed permanently. This can’t be undone.</p>
    {running && <p class="inline-error">pi is working in this project. Stop the run before deleting it.</p>}
    {error && <p class="inline-error" role="alert">{error}</p>}
    <div class="dialog-actions">
      <button class="button secondary" onClick={onClose} disabled={deleting}>Cancel</button>
      <button class="button danger" onClick={confirm} disabled={deleting || running}><Icon name="trash" size={16}/>{deleting ? 'Deleting…' : 'Delete project'}</button>
    </div>
  </Dialog>;
}
