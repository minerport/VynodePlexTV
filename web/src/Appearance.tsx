import { useState } from "react";
export const themes = ["Tidal", "Aurora", "Ember", "Glacier", "OLED"] as const;
type Appearance = { theme: string; compact: boolean; effects: boolean };
const defaults: Appearance = { theme: "Tidal", compact: false, effects: true };
export function readAppearance(): Appearance {
  try {
    const value = JSON.parse(
      localStorage.getItem("afterglow.appearance") || "{}",
    );
    return {
      theme: themes.includes(value.theme) ? value.theme : defaults.theme,
      compact: value.compact === true,
      effects: value.effects !== false,
    };
  } catch {
    return defaults;
  }
}
export function applyAppearance(value: Appearance) {
  document.documentElement.dataset.theme = value.theme.toLowerCase();
  document.documentElement.dataset.density = value.compact
    ? "compact"
    : "cinematic";
  document.documentElement.dataset.effects = value.effects ? "on" : "off";
}
export function AppearanceSettings() {
  const [value, setValue] = useState(readAppearance);
  function update(next: Appearance) {
    setValue(next);
    applyAppearance(next);
    try {
      localStorage.setItem("afterglow.appearance", JSON.stringify(next));
    } catch {
      /* Appearance still applies for this visit. */
    }
  }
  return (
    <section
      className="appearance-panel settings-panel"
      aria-label="Appearance"
    >
      <p className="eyebrow">MAKE IT YOURS</p>
      <h2>Choose your atmosphere</h2>
      <p>
        Appearance is saved in this browser. Your programming stays the same.
      </p>
      <div className="theme-gallery">
        {themes.map((theme) => (
          <button
            key={theme}
            data-swatch={theme.toLowerCase()}
            aria-pressed={value.theme === theme}
            onClick={() => update({ ...value, theme })}
          >
            <span className="theme-sample" aria-hidden="true" />
            <strong>{theme}</strong>
            <span>{value.theme === theme ? "Selected" : "Preview theme"}</span>
          </button>
        ))}
      </div>
      <div className="appearance-options">
        <button
          aria-pressed={value.compact}
          onClick={() => update({ ...value, compact: !value.compact })}
        >
          Layout: {value.compact ? "Compact" : "Cinematic"}
        </button>
        <button
          aria-pressed={value.effects}
          onClick={() => update({ ...value, effects: !value.effects })}
        >
          Effects: {value.effects ? "Ambient" : "Reduced"}
        </button>
      </div>
    </section>
  );
}
