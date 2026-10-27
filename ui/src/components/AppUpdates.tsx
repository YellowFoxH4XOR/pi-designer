import { useEffect, useState } from 'preact/hooks';

type UpdateSettings = {
  version: string;
  configured: boolean;
  canCheck: boolean;
  busy: boolean;
  automaticChecks: boolean;
  automaticDownloads: boolean;
  message: string;
};

function send(action: string, preferences?: Partial<UpdateSettings>) {
  window.webkit?.messageHandlers?.pidesigner?.postMessage({ action, preferences });
}

export function AppUpdates() {
  const native = !!window.__pidesignerNative;
  const [settings, setSettings] = useState<UpdateSettings | null>(null);
  const [failure, setFailure] = useState('');
  useEffect(() => {
    if (!native) return;
    const timeout = window.setTimeout(() => setFailure('The Mac update service did not respond. Reopen Settings to try again.'), 8000);
    const receive = (event: Event) => {
      window.clearTimeout(timeout);
      setFailure('');
      setSettings((event as CustomEvent<UpdateSettings>).detail);
    };
    window.addEventListener('pidesigner:update-settings', receive);
    send('getUpdateSettings');
    return () => {
      window.clearTimeout(timeout);
      window.removeEventListener('pidesigner:update-settings', receive);
    };
  }, [native]);
  return <section class="settings-section" aria-labelledby="app-updates-heading">
    <div class="settings-section-heading"><h2 id="app-updates-heading">App updates</h2><p>Keep πDesigner up to date with signed releases from GitHub.</p></div>
    {!native ? <p class="settings-note">App updates are available in the πDesigner Mac app.</p>
      : failure ? <p class="inline-error" role="alert">{failure}</p>
      : !settings ? <p class="loading-state" role="status">Reading app update settings…</p>
      : <div class="native-preferences">
        <p>Installed version <code>{settings.version}</code></p>
        <p class="native-permission" role="status">{settings.message}</p>
        <div><button class="button secondary" disabled={!settings.canCheck} onClick={() => send('checkForUpdates')}>{settings.busy ? 'Show update progress…' : 'Check for updates…'}</button></div>
        <label><input type="checkbox" checked={settings.automaticChecks} disabled={!settings.configured} onChange={e => send('updateUpdateSettings', { automaticChecks: e.currentTarget.checked })}/>Automatically check for new releases</label>
        <label><input type="checkbox" checked={settings.automaticDownloads} disabled={!settings.configured || !settings.automaticChecks} onChange={e => send('updateUpdateSettings', { automaticDownloads: e.currentTarget.checked })}/>Automatically download and install updates when I quit</label>
        <p class="settings-note">Checks run daily. Downloads are verified before installation. Updates preserve your projects; restarting still asks before stopping active tasks or discarding unsaved edits.</p>
      </div>}
  </section>;
}
