import { Call, Events } from "/wails/runtime.js";

export { Events };

/** Calls a Go method on TvService. */
export const tv = (method, ...args) => Call.ByName("main.TvService." + method, ...args);
/** Calls a Go method on InstallerService. */
export const installer = (method, ...args) => Call.ByName("main.InstallerService." + method, ...args);

export const $ = (sel, root = document) => root.querySelector(sel);

export function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k.startsWith("on")) e.addEventListener(k.slice(2), v);
    else if (k === "class") e.className = v;
    else if (v === true) e.setAttribute(k, "");
    else if (v !== false && v != null) e.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c != null) e.append(c);
  }
  return e;
}

let toastTimer;
export function toast(msg) {
  let t = $(".toast");
  if (!t) document.body.append((t = el("div", { class: "toast" })));
  t.textContent = String(msg?.message ?? msg);
  t.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove("show"), 3500);
}

/** Runs fn, showing any error as a toast. */
export async function safe(fn) {
  try {
    return await fn();
  } catch (e) {
    console.error(e);
    toast(e);
  }
}

export function debounce(fn, ms) {
  let t;
  return (...a) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...a), ms);
  };
}

/** Rate-limits fn to one call per ms, always delivering the last value. */
export function throttle(fn, ms) {
  let last = 0, timer, pending;
  return (...a) => {
    pending = a;
    const wait = last + ms - Date.now();
    if (wait <= 0) {
      last = Date.now();
      fn(...pending);
    } else if (!timer) {
      timer = setTimeout(() => {
        timer = null;
        last = Date.now();
        fn(...pending);
      }, wait);
    }
  };
}

export const powerIcon = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M12 3v8"/><path d="M6.3 6.8a8 8 0 1 0 11.4 0"/></svg>';
