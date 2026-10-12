// Key-sequence editor: keys shown as removable chips, added from a mini remote.
import { tv, el, safe } from "/common.js";

const LABELS = {
  KEY_MENU: "Menu", KEY_HOME: "Home", KEY_SOURCE: "Source", KEY_TOOLS: "Tools", KEY_INFO: "Info",
  KEY_UP: "▲", KEY_DOWN: "▼", KEY_LEFT: "◀", KEY_RIGHT: "▶", KEY_ENTER: "OK",
  KEY_RETURN: "↩ Назад", KEY_EXIT: "Exit", KEY_GUIDE: "Guide", KEY_CH_LIST: "CH List",
};
export const keyLabel = k => LABELS[k] ?? k.replace(/^KEY_/, "");

const LAYOUT = [
  ["KEY_MENU", "KEY_HOME", "KEY_SOURCE", "KEY_TOOLS"],
  [null, "KEY_UP", null],
  ["KEY_LEFT", "KEY_ENTER", "KEY_RIGHT"],
  [null, "KEY_DOWN", null],
  ["KEY_RETURN", "KEY_EXIT", "KEY_INFO"],
];

let openPopover = null;
function closePopover() {
  openPopover?.remove();
  openPopover = null;
}
document.addEventListener("pointerdown", e => {
  if (openPopover && !openPopover.contains(e.target) && !e.target.closest(".keyseq")) closePopover();
});
document.addEventListener("keydown", e => { if (e.key === "Escape") closePopover(); });

/**
 * keySeqField renders an editor for a list of remote keys.
 * @param {{mac: string, get: () => string[], set: (keys: string[]) => void}} o
 */
export function keySeqField({ mac, get, set }) {
  const field = el("div", { class: "keyseq", tabindex: 0, title: "Нажмите, чтобы добавить кнопки" });

  const render = () => {
    const keys = get();
    field.replaceChildren(
      ...keys.map((k, i) => el("span", { class: "chip", title: k },
        keyLabel(k),
        el("button", { class: "x", title: "Удалить", onclick: e => {
          e.stopPropagation();
          set(keys.filter((_, j) => j !== i));
          render();
        } }, "×"))),
      el("span", { class: "add" }, keys.length ? "+" : "+ добавить кнопки"));
  };

  const append = k => { set([...get(), k]); render(); };

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
      el("label", { class: "check small" }, live, "Сразу нажимать на ТВ"),
      el("div", { class: "minirc-row" },
        el("button", { onclick: () => { set([]); render(); } }, "Очистить"),
        el("button", { class: "primary", onclick: closePopover }, "Готово")));
    pop.owner = field;
    document.body.append(pop);
    const r = field.getBoundingClientRect();
    const top = r.bottom + window.scrollY + 4;
    const left = Math.min(r.left + window.scrollX, document.documentElement.clientWidth - pop.offsetWidth - 8);
    pop.style.top = top + "px";
    pop.style.left = Math.max(8, left) + "px";
    openPopover = pop;
    pop.scrollIntoView({ block: "nearest" });
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
