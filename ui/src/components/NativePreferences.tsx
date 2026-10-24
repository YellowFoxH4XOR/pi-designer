import { useEffect, useState } from "preact/hooks";

type Preferences = Record<
  "notch" | "completed" | "attention" | "failure" | "sound" | "previews",
  boolean
> & { authorization: "authorized" | "denied" | "notDetermined" };
declare global {
  interface Window {
    __pidesignerNative?: boolean;
    webkit?: {
      messageHandlers?: {
        pidesigner?: { postMessage: (message: unknown) => void };
      };
    };
  }
}
function send(action: string, preferences?: Partial<Preferences>) {
  window.webkit?.messageHandlers?.pidesigner?.postMessage({
    action,
    preferences,
  });
}
export function NativePreferences() {
  const [settings, setSettings] = useState<Preferences | null>(null);
  const [waiting, setWaiting] = useState(false);
  const native = !!window.__pidesignerNative;
  useEffect(() => {
    const receive = (e: Event) => {
      setSettings((e as CustomEvent<Preferences>).detail);
      setWaiting(false);
    };
    window.addEventListener("pidesigner:native-settings", receive);
    if (native) send("getSettings");
    return () =>
      window.removeEventListener("pidesigner:native-settings", receive);
  }, [native]);
  const labels: [keyof Omit<Preferences, "authorization">, string][] = [
    ["notch", "Show notch activity when the workspace is in the background"],
    ["completed", "Notify when a task finishes"],
    ["attention", "Notify when pi needs my answer"],
    ["failure", "Notify when a run fails"],
    ["sound", "Play notification sounds"],
    ["previews", "Include project names in notifications"],
  ];
  return (
    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>Mac activity & notifications</h2>
        <p>
          Close or minimize the window without stopping your work. Quit stops
          active tasks.
        </p>
      </div>
      {!native ? (
        <p class="native-permission">
          These controls are available in the πDesigner Mac app.
        </p>
      ) : !settings ? (
        <p class="native-permission" role="status">
          Reading Mac notification preferences…
        </p>
      ) : (
        <div class="native-preferences">
          <p class="native-permission">
            {settings.authorization === "authorized"
              ? "macOS notifications are allowed."
              : settings.authorization === "denied"
                ? "Notifications are blocked. Allow πDesigner in macOS System Settings → Notifications."
                : "Allow notifications to hear when your work finishes or needs attention."}
          </p>
          {settings.authorization === "notDetermined" && (
            <div>
              <button
                class="button secondary"
                disabled={waiting}
                onClick={() => {
                  setWaiting(true);
                  send("requestNotifications");
                }}
              >
                {waiting ? "Waiting for macOS…" : "Enable Mac notifications"}
              </button>
            </div>
          )}
          {labels.map(([key, label]) => (
            <label key={key}>
              <input
                type="checkbox"
                checked={settings[key]}
                onChange={(e) => {
                  const checked = e.currentTarget.checked;
                  setSettings({ ...settings, [key]: checked });
                  send("updateSettings", { [key]: checked });
                }}
              />
              {label}
            </label>
          ))}
          <p class="native-permission">
            The menu bar always provides access. The notch panel stays below the
            camera/menu bar and appears only while work is active or needs
            attention. Notification previews are private by default.
          </p>
        </div>
      )}
    </section>
  );
}
