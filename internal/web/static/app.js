let sentinelObserver = null;

function observeSentinel() {
  const body = document.getElementById("body-conversation");
  const sentinel = document.querySelector(".sentinel-top");
  if (sentinelObserver) sentinelObserver.disconnect();
  if (!body || !sentinel) return;
  sentinelObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) htmx.trigger(entry.target, "load-more");
      }
    },
    { root: body }
  );
  sentinelObserver.observe(sentinel);
}

document.body.addEventListener("htmx:afterSettle", observeSentinel);
document.addEventListener("DOMContentLoaded", observeSentinel);

let heightBeforeSwap = 0;

document.body.addEventListener("htmx:beforeSwap", (event) => {
  if (!event.target.classList.contains("sentinel-top")) return;
  const body = document.getElementById("body-conversation");
  heightBeforeSwap = body ? body.scrollHeight : 0;
});

document.body.addEventListener("htmx:afterSwap", () => {
  const body = document.getElementById("body-conversation");
  if (!body || !heightBeforeSwap) return;
  body.scrollTop += body.scrollHeight - heightBeforeSwap;
  heightBeforeSwap = 0;
});

function layoutChipFilters() {
  const bar = document.querySelector(".filters-chats");
  if (!bar) return;
  const row = bar.querySelector(".row-chips");
  let chips = [...row.querySelectorAll(".chip-list-wrap")];
  chips.forEach((chip) => (chip.hidden = false));

  // The active list is pulled to the front so it can never be the one hidden.
  const active = row.querySelector(".chip-list-wrap:has(.chip-filter--active)");
  if (active && active !== chips[0]) {
    row.insertBefore(active, chips[0]);
    chips = [...row.querySelectorAll(".chip-list-wrap")];
  }

  // Measure every chip before hiding any: hiding shifts the ones after it.
  const limit = row.getBoundingClientRect().right;
  const rights = chips.map((chip) => chip.getBoundingClientRect().right);
  let overflowFrom = chips.length;
  for (let i = 0; i < chips.length; i++) {
    if (rights[i] > limit) { overflowFrom = i; break; }
  }
  // Contiguous cut: once one chip overflows, every chip after it goes too.
  chips.forEach((chip, i) => (chip.hidden = i >= overflowFrom));
  bar.querySelectorAll(".item-menu-list").forEach((item) => {
    const chip = chips.find((c) => c.dataset.listId === item.dataset.listId);
    item.hidden = !(chip && chip.hidden);
  });
}

document.addEventListener("DOMContentLoaded", layoutChipFilters);
window.addEventListener("resize", layoutChipFilters);
document.body.addEventListener("htmx:afterSettle", layoutChipFilters);

// Held here and not in the alert box's Alpine state: Alpine calls any function
// it finds while evaluating an expression, so a callback kept there would fire
// as soon as the box rendered.
let pendingConfirm = null;
window.addEventListener("confirm-accepted", () => {
  const run = pendingConfirm;
  pendingConfirm = null;
  if (run) run();
});

document.body.addEventListener("htmx:confirm", (event) => {
  if (!(event.detail.path || "").endsWith("/fixar")) return;
  if (event.detail.elt.dataset.pinned === "1") return;
  const strip = document.querySelector(".banner-pinned");
  if (!strip || Number(strip.dataset.total) < Number(strip.dataset.limit)) return;
  event.preventDefault();
  pendingConfirm = () => event.detail.issueRequest(true);
  window.dispatchEvent(new CustomEvent("confirm", { detail: { text: strip.dataset.confirm } }));
});

document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.target.id !== "conversation" && event.target.id !== "list-messages") return;
  const body = document.getElementById("body-conversation");
  if (!body) return;
  const highlighted = body.querySelector(".row-highlighted");
  if (highlighted) {
    highlighted.scrollIntoView({ block: "start" });
  } else {
    body.scrollTop = body.scrollHeight;
  }
});

// The lightbox lives in layout.html, outside #conversation, so swapping
// #conversation never tears it down on its own.
document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.target.id !== "conversation") return;
  window.dispatchEvent(new CustomEvent("close-lightbox"));
});

window.pageLimit = null;

// The rail sits outside every swap target, so nothing else would mark which destination is
// on screen. The template owns which button answers for which path (data-rail-path).
function markRail(path) {
  document.querySelectorAll("#rail .rail-active").forEach((e) => e.classList.remove("rail-active"));
  const button = document.querySelector(`#rail [data-rail-path="${CSS.escape(path)}"]`);
  if (button) button.classList.add("rail-active");
}

// A destination reached by deep link or reload never swaps, so the mark has to be set from
// the URL too, not only from htmx.
markRail(location.pathname);

// Only a swap into #conversation counts: that is what a destination replaces, while
// /imports/badge and /imports/progress keep polling in the background.
document.body.addEventListener("htmx:afterSwap", (event) => {
  if (!event.detail.target || event.detail.target.id !== "conversation") return;
  markRail((event.detail.pathInfo && event.detail.pathInfo.requestPath || "").split("?")[0]);
});

// The badge polls every 5s from any screen, and carries a counter that changes when an
// import brought messages in. That is what makes a conversation appear in the sidebar even
// when the user left the imports page — the progress bar's own reload dies with that page.
document.body.addEventListener("htmx:afterSwap", (event) => {
  const mark = event.target.querySelector && event.target.querySelector(".mark-imported");
  if (!mark) return;
  const value = mark.dataset.imported;
  if (window.importedMark !== undefined && window.importedMark !== value) {
    htmx.ajax("GET", window.lastChatList || "/chats", "#list-chats");
  }
  window.importedMark = value;
});

// The chat list's own filter (chip, list, search term) lives in the URL that produced it.
// Remembering the last one lets the reload after an import land on the same view instead of
// throwing the user back to "Tudo".
document.body.addEventListener("htmx:afterRequest", (event) => {
  const path = event.detail.pathInfo && event.detail.pathInfo.finalRequestPath;
  const target = event.detail.target;
  if (target && target.id === "list-chats" && path && path.startsWith("/chats?")) {
    window.lastChatList = path;
  }
});

document.body.addEventListener("htmx:configRequest", (event) => {
  if (event.detail.elt && event.detail.elt.dataset.restoreChatList && window.lastChatList) {
    event.detail.path = window.lastChatList;
  }
});

document.body.addEventListener("htmx:configRequest", (event) => {
  const detail = event.detail;
  if (!detail.path.startsWith("/chats/")) return;

  // Only opening a conversation resets the page size; subpaths keep it.
  if (/^\/chats\/\d+$/.test(detail.path)) {
    window.pageLimit = null;
    return;
  }

  const fromForm = detail.parameters && detail.parameters.limit;
  const fromURL = detail.path.match(/[?&]limit=(\d+)/);
  if (fromForm || fromURL) {
    window.pageLimit = fromForm || fromURL[1];
    return;
  }
  if (!window.pageLimit) return;
  // In the path, not in parameters: on a POST htmx would send it in the body.
  detail.path += (detail.path.includes("?") ? "&" : "?") + "limit=" + encodeURIComponent(window.pageLimit);
});

const AUDIO_SPEEDS = [1, 1.5, 2];

function audioVoice() {
  return {
    playing: false,
    duration: 0,
    position: 0,
    speed: 1,
    get progress() {
      return this.duration ? (this.position / this.duration) * 100 : 0;
    },
    get currentTime() {
      return formatAudioTime(this.position);
    },
    get totalTime() {
      return formatAudioTime(this.duration);
    },
    get displayedSpeed() {
      return this.speed.toFixed(1).replace(".", ",") + "x";
    },
    play() {
      this.$refs.audio.play();
      this.playing = true;
    },
    pause() {
      this.$refs.audio.pause();
      this.playing = false;
    },
    seek(event) {
      if (!this.duration) return;
      const box = event.currentTarget.getBoundingClientRect();
      const ratio = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
      this.$refs.audio.currentTime = ratio * this.duration;
    },
    cycleSpeed() {
      const next = AUDIO_SPEEDS[(AUDIO_SPEEDS.indexOf(this.speed) + 1) % AUDIO_SPEEDS.length];
      this.speed = next;
      this.$refs.audio.playbackRate = next;
    },
  };
}

function formatAudioTime(seconds) {
  const total = Number.isFinite(seconds) ? Math.floor(seconds) : 0;
  const m = Math.floor(total / 60);
  const s = String(total % 60).padStart(2, "0");
  return `${m}:${s}`;
}

// One player at a time. Capture phase because "play" doesn't bubble.
document.addEventListener(
  "play",
  (event) => {
    for (const media of document.querySelectorAll("video, audio")) {
      if (media !== event.target) media.pause();
    }
  },
  true
);

// 4px per bar: 2px bar + 2px gap.
const WAVEFORM_STEP = 4;
const WAVEFORM_DEFAULT_BARS = 36;

function countBars(wrap) {
  const width = wrap.clientWidth;
  if (!width) return WAVEFORM_DEFAULT_BARS;
  return Math.max(8, Math.floor((width + 2) / WAVEFORM_STEP));
}

const waveformObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      waveformObserver.unobserve(entry.target);
      drawRealWaveform(entry.target);
    }
  },
  { rootMargin: "300px" }
);

function observeWaveforms() {
  document.querySelectorAll(".audio-voice-waveform:not([data-waveform-observed])").forEach((el) => {
    el.setAttribute("data-waveform-observed", "1");
    waveformObserver.observe(el);
  });
}

document.body.addEventListener("htmx:afterSettle", observeWaveforms);
document.addEventListener("DOMContentLoaded", observeWaveforms);

async function drawRealWaveform(wrap) {
  const audio = wrap.closest(".audio-voice")?.querySelector("audio");
  if (!audio) return;
  try {
    const response = await fetch(audio.currentSrc || audio.src);
    const buffer = await response.arrayBuffer();
    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    const context = new AudioContextClass();
    const decoded = await context.decodeAudioData(buffer);
    context.close();

    const bars = countBars(wrap);
    const waveformWidth = bars * WAVEFORM_STEP - 2;

    const data = decoded.getChannelData(0);
    const chunkSize = Math.max(1, Math.floor(data.length / bars));
    const peaks = [];
    let maxPeak = 0;
    for (let i = 0; i < bars; i++) {
      let peak = 0;
      const start = i * chunkSize;
      const end = Math.min(data.length, start + chunkSize);
      for (let j = start; j < end; j++) peak = Math.max(peak, Math.abs(data[j]));
      peaks.push(peak);
      maxPeak = Math.max(maxPeak, peak);
    }
    if (maxPeak === 0) return;

    const minHeight = 3;
    const maxHeight = 18;
    const rects = peaks
      .map((peak, i) => {
        const height = minHeight + (peak / maxPeak) * (maxHeight - minHeight);
        const y = ((22 - height) / 2).toFixed(1);
        return `<rect x="${i * WAVEFORM_STEP}" y="${y}" width="2" height="${height.toFixed(1)}" rx="1"/>`;
      })
      .join("");

    wrap.querySelectorAll(":scope > svg, :scope > .audio-voice-waveform-progress > svg").forEach((svg) => {
      svg.setAttribute("width", waveformWidth);
      svg.setAttribute("viewBox", `0 0 ${waveformWidth} 22`);
      svg.innerHTML = rects;
    });
  } catch {
    // No decodeAudioData support, or the fetch failed: keep the static waveform.
  }
}

document.body.addEventListener("htmx:afterRequest", (event) => {
  const path = event.detail.pathInfo?.requestPath || "";
  if (!event.detail.successful || !/\/(renomear|avatar)$/.test(path)) return;
  const search = document.querySelector(".field-search");
  if (search) htmx.trigger(search, "search");
});

document.body.addEventListener("change", (event) => {
  // Any input marked with data-cropper-for feeds the cropper; the attribute says which
  // hidden input receives the cropped blob, and so which route the POST goes to.
  if (event.target.id !== "input-avatar-raw" && !event.target.dataset.cropperFor) return;
  window.cropperTarget = event.target.dataset.cropperFor || "input-avatar-final";
  const file = event.target.files[0];
  event.target.value = ""; // lets the same file be picked again later
  if (!file) return;
  const reader = new FileReader();
  reader.onload = () => {
    window.dispatchEvent(new CustomEvent("open-cropper", { detail: { dataUrl: reader.result } }));
  };
  reader.readAsDataURL(file);
});

async function cropperToBlob(imgEl, x, y, scale, windowPx, maxOutputPx) {
  await imgEl.decode().catch(() => {});
  const naturalWidth = imgEl.naturalWidth;
  const naturalHeight = imgEl.naturalHeight;
  if (!naturalWidth || !naturalHeight) return null;

  // Cover-fit on the SMALLER natural dimension, or a landscape photo leaves the
  // circle's top and bottom uncovered. Must match the :style on the <img> in
  // layout.html, or the crop won't line up with what was on screen.
  const coverBasis = naturalWidth > naturalHeight ? naturalHeight : naturalWidth;
  const k = (windowPx / coverBasis) * scale;
  const scaledWidth = naturalWidth * k;
  const scaledHeight = naturalHeight * k;
  const imgLeft = windowPx / 2 + x - scaledWidth / 2;
  const imgTop = windowPx / 2 + y - scaledHeight / 2;

  // windowPx / k is how many natural pixels are inside the window right now.
  // A fixed output size would throw away real detail from a good photo, leaving
  // the lightbox a small master to upscale. Capped, and never upscaled.
  const outputPx = Math.round(Math.min(maxOutputPx, Math.max(64, windowPx / k)));

  const canvas = document.createElement("canvas");
  canvas.width = outputPx;
  canvas.height = outputPx;
  const ctx = canvas.getContext("2d");
  ctx.drawImage(imgEl, -imgLeft / k, -imgTop / k, windowPx / k, windowPx / k, 0, 0, outputPx, outputPx);

  return new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
}

document.body.addEventListener("click", (event) => {
  const item = event.target.closest(".item-chat");
  if (!item) return;
  document.querySelectorAll(".item-chat--active").forEach((el) => el.classList.remove("item-chat--active"));
  item.classList.add("item-chat--active");
});
