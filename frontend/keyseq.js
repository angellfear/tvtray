// Key-sequence editor: chips can be dragged to reorder and clicked to set a
// per-key pause; new keys come from a mini remote.
import { tv, el, safe } from "/common.js";
import { t } from "/i18n.js";

const LABELS = {
  KEY_MENU: "Menu", KEY_HOME: "Home", KEY_SOURCE: "Source", KEY_TOOLS: "Tools", KEY_INFO: "Info",
  KEY_UP: "▲", KEY_DOWN: "▼", KEY_LEFT: "◀", KEY_RIGHT: "▶", KEY_ENTER: "OK",
  KEY_EXIT: "Exit", KEY_GUIDE: "Guide", KEY_CH_LIST: "CH List",
};
export const keyLabel = k => k === "KEY_RETURN" ? t("key.back") : LABELS[k] ?? k.replace(/^KEY_/, "");

const LAYOUT = [
  ["KEY_MENU", "KEY_HOME", "KEY_SOURCE", "KEY_TOOLS"],
  [null, "KEY_UP", null],
  ["KEY_LEFT", "KEY_ENTER", "KEY_RIGHT"],
  [null, "KEY_DOWN", null],
  ["KEY_RETURN", "KEY_EXIT", "KEY_INFO"],
];

const MAX_PAUSE = 120000;

let openPopover = null;
function closePopover() {
  openPopover?.remove();
  openPopover = null;
}
document.addEventListener("pointerdown", e => {
  if (openPopover && !openPopover.contains(e.target) && !e.target.closest(".keyseq")) closePopover();
});
document.addEventListener("keydown", e => { if (e.key === "Escape") closePopover(); });

function anchorPopover(pop, at) {
  document.body.append(pop);
  const r = at.getBoundingClientRect();
  const top = r.bottom + window.scrollY + 4;
  const left = Math.min(r.left + window.scrollX, document.documentElement.clientWidth - pop.offsetWidth - 8);
  pop.style.top = top + "px";
  pop.style.left = Math.max(8, left) + "px";
  openPopover = pop;
  pop.scrollIntoView({ block: "nearest" });
}

/**
 * keySeqField renders an editor for a list of macro steps. A step is
 * { key, pause } — pause in ms, 0 = use the default pause between presses.
 * @param {{mac: string, get: () => Array<{key: string, pause?: number}>, set: (steps: any[]) => void}} o
 */
export function keySeqField({ mac, get, set }) {
  const field = el("div", { class: "keyseq", tabindex: 0, title: t("ks.field") });
  let dragIdx = -1;

  // pausePopover edits the pause of one chip; the value replaces the default.
  const pausePopover = (at, step, apply) => {
    const input = el("input", {
      type: "number", min: 0, max: MAX_PAUSE, step: 50,
      value: step.pause || "", placeholder: t("ks.pauseHint"), style: "width: 100%",
    });
    const applyVal = () => {
      apply(input.value === "" ? 0 : Math.min(MAX_PAUSE, Math.max(0, Math.trunc(+input.value) || 0)));
      closePopover();
    };
    input.addEventListener("keydown", e => { if (e.key === "Enter") { e.preventDefault(); applyVal(); } });
    const pop = el("div", { class: "minirc" },
      el("div", { class: "minirc-row" }, el("b", {}, keyLabel(step.key))),
      el("div", { class: "small muted" }, t("ks.pause")),
      el("div", { class: "minirc-row" }, input),
      el("div", { class: "minirc-row" },
        el("button", { title: t("ks.pauseHint"), onclick: () => { input.value = ""; applyVal(); } }, t("ks.reset")),
        el("button", { class: "primary", onclick: applyVal }, t("ks.done"))));
    pop.owner = at;
    anchorPopover(pop, at);
    input.focus();
  };

  const render = () => {
    const steps = get();
    field.replaceChildren(
      ...steps.map((s, i) => {
        const chip = el("span", { class: "chip", title: s.key },
          keyLabel(s.key),
          s.pause ? el("span", { class: "pause" }, "⏱" + s.pause) : null,
          el("button", { class: "x", title: t("ks.remove"), onclick: e => {
            e.stopPropagation();
            set(steps.filter((_, j) => j !== i));
            render();
          } }, "×"));
        chip.draggable = true;
        chip.addEventListener("dragstart", e => {
          dragIdx = i;
          e.dataTransfer.effectAllowed = "move";
          chip.classList.add("dragging");
        });
        chip.addEventListener("dragend", () => chip.classList.remove("dragging"));
        chip.addEventListener("dragover", e => {
          e.preventDefault();
          e.dataTransfer.dropEffect = "move";
        });
        chip.addEventListener("drop", e => {
          e.preventDefault();
          e.stopPropagation();
          if (dragIdx < 0 || dragIdx === i) return;
          const next = [...steps];
          const [moved] = next.splice(dragIdx, 1);
          next.splice(i, 0, moved);
          dragIdx = -1;
          set(next);
          render();
        });
        chip.addEventListener("click", e => {
          e.stopPropagation();
          if (openPopover?.owner === chip) { closePopover(); return; }
          closePopover();
          pausePopover(chip, s, v => {
            const next = [...steps];
            next[i] = { key: s.key, pause: v };
            set(next);
            render();
          });
        });
        return chip;
      }),
      el("span", { class: "add" }, steps.length ? "+" : t("ks.add")));
  };

  const append = k => { set([...get(), { key: k }]); render(); };

  field.addEventListener("click", () => {
    if (openPopover?.owner === field) return;
    closePopover();
    const live = el("input", { type: "checkbox", checked: true });
    const press = k => {
      append(k);
      if (live.checked) safe(() => tv("Key", mac, k));
    };
    const pop = el("div", { class: "minirc" },
      ...LAYOUT.map(row => el("div", { class: "minirc-row" + (row.length === 3 ? " three" : "") },
        row.map(k => k ? el("button", { class: k === "KEY_ENTER" ? "ok" : "", onclick: () => press(k) }, keyLabel(k)) : el("span")))),
      el("label", { class: "check small" }, live, t("ks.live")),
      el("div", { class: "minirc-row" },
        el("button", { onclick: () => { set([]); render(); } }, t("ks.clear")),
        el("button", { class: "primary", onclick: closePopover }, t("ks.done"))));
    pop.owner = field;
    anchorPopover(pop, field);
  });

  field.addEventListener("keydown", e => {
    if (e.key === "Backspace") {
      e.preventDefault();
      set(get().slice(0, -1));
      render();
    }
  });

  render();
  return field;
}
