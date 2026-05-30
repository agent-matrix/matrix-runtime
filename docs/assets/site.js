/* ============================================================
   Matrix Cloud site — cinematic effects + interactions.
   Dependency-free vanilla JS. Adapted from the MatrixHub fx engine.
   ============================================================ */
(function () {
  "use strict";

  /* ---------- Canvas digital rain ---------- */
  const RAIN_GLYPHS = 'ﾊﾐﾋｰｳｼﾅﾓﾆｻﾜｵﾘｱﾎﾃﾏｹﾒｴｶｷﾑﾕﾗｾﾈｦｲｸｺｿﾁﾄﾉｧｨｩｪｫｬｭｮ012345789Z:."=*+-<>¦｜╌';

  function startMatrixRain(canvas) {
    const ctx = canvas.getContext && canvas.getContext("2d", { alpha: true });
    if (!ctx) return;
    const FONT = 16;
    let width = 0, height = 0, columns = 0, drops = [], speeds = [], dpr = 1, cache = [];

    function resize() {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      width = canvas.clientWidth; height = canvas.clientHeight;
      canvas.width = Math.floor(width * dpr); canvas.height = Math.floor(height * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      columns = Math.ceil(width / FONT);
      drops = new Array(columns).fill(0).map(() => Math.random() * -50);
      speeds = new Array(columns).fill(0).map(() => 0.06 + Math.random() * 0.16);
      ctx.font = FONT + 'px "JetBrains Mono", monospace';
      ctx.textBaseline = "top";
    }
    function glyph(i, row) {
      const key = i * 997 + row;
      if (!cache[key] || Math.random() < 0.012) cache[key] = RAIN_GLYPHS[(Math.random() * RAIN_GLYPHS.length) | 0];
      return cache[key];
    }
    function frame() {
      ctx.fillStyle = "rgba(0, 4, 2, 0.085)";
      ctx.fillRect(0, 0, width, height);
      for (let i = 0; i < columns; i++) {
        const x = i * FONT, y = drops[i] * FONT;
        if (y > 0 && y < height) {
          ctx.fillStyle = "rgba(210, 255, 223, 0.95)";
          ctx.fillText(glyph(i, Math.floor(drops[i])), x, y);
          ctx.fillStyle = "rgba(0, 255, 102, 0.55)";
          ctx.fillText(glyph(i, Math.floor(drops[i]) - 1), x, y - FONT);
        }
        drops[i] += speeds[i];
        if (y > height && Math.random() > 0.975) { drops[i] = Math.random() * -30; speeds[i] = 0.06 + Math.random() * 0.16; }
      }
      requestAnimationFrame(frame);
    }
    resize();
    let rt; window.addEventListener("resize", () => { clearTimeout(rt); rt = setTimeout(resize, 150); });
    requestAnimationFrame(frame);
  }

  /* ---------- Decode text reveal ---------- */
  const SCRAMBLE = "ｱｲｳｴｵｶｷｸ01<>/\\=+*ﾊﾐﾋ█▓▒░";
  function decode(el, duration, delay) {
    const text = el.getAttribute("data-decode") || el.textContent;
    const chars = text.split("");
    let start = null;
    function step(ts) {
      if (start === null) start = ts;
      const elapsed = ts - start;
      if (elapsed < delay) { requestAnimationFrame(step); return; }
      const p = Math.min(1, (elapsed - delay) / duration);
      const revealed = Math.floor(p * chars.length);
      let out = "";
      for (let i = 0; i < chars.length; i++) {
        if (chars[i] === " ") { out += " "; continue; }
        out += i < revealed ? chars[i] : SCRAMBLE[(Math.random() * SCRAMBLE.length) | 0];
      }
      el.textContent = out;
      if (p < 1) requestAnimationFrame(step); else el.textContent = text;
    }
    requestAnimationFrame(step);
  }

  /* ---------- Reveal on scroll ---------- */
  function revealOnScroll() {
    const els = document.querySelectorAll(".reveal");
    if (!("IntersectionObserver" in window)) { els.forEach((e) => e.classList.add("in")); return; }
    const io = new IntersectionObserver((entries) => {
      entries.forEach((en) => {
        if (en.isIntersecting) {
          const d = parseInt(en.target.getAttribute("data-delay") || "0", 10);
          setTimeout(() => en.target.classList.add("in"), d);
          io.unobserve(en.target);
        }
      });
    }, { threshold: 0.12 });
    els.forEach((e) => io.observe(e));
  }

  /* ---------- Install tabs ---------- */
  const INSTALL = {
    "Local": '<span class="c-prompt">$ </span><span class="c-cmd">make build</span>\n<span class="c-prompt">$ </span><span class="c-cmd">./bin/matrix-runtime</span> <span class="c-flag">--mode local-dev</span>\n<span class="c-cmt"># open the console at http://localhost:8080</span>',
    "Docker": '<span class="c-prompt">$ </span><span class="c-cmd">docker run</span> <span class="c-flag">-d -p 8080:8080</span> \\\n    <span class="c-flag">-e MATRIX_RUNTIME_MODE=customer-agent</span> \\\n    <span class="c-flag">-e MATRIX_RUNTIME_JOIN_TOKEN=mxrt_xxxxx</span> \\\n    ghcr.io/agent-matrix/matrix-runtime:latest',
    "Helm": '<span class="c-prompt">$ </span><span class="c-cmd">helm install</span> matrix-runtime ./deploy/helm/matrix-runtime \\\n    <span class="c-flag">--namespace</span> matrix-runtime <span class="c-flag">--create-namespace</span> \\\n    <span class="c-flag">--set</span> cloud.url=https://cloud.matrixhub.io \\\n    <span class="c-flag">--set</span> runtime.joinToken=mxrt_xxxxx',
    "HF Space": '<span class="c-cmt"># Duplicate the Space, then set secrets:</span>\n<span class="c-cmd">MATRIX_RUNTIME_MODE</span>=hf-space\n<span class="c-cmd">MATRIX_CLOUD_URL</span>=https://cloud.matrixhub.io\n<span class="c-cmd">MATRIX_RUNTIME_JOIN_TOKEN</span>=mxrt_xxxxx',
  };
  function setupInstall() {
    const tabsEl = document.getElementById("install-tabs");
    const codeEl = document.getElementById("install-code");
    if (!tabsEl || !codeEl) return;
    Object.keys(INSTALL).forEach((k, i) => {
      const b = document.createElement("button");
      b.className = "tab" + (i === 0 ? " active" : "");
      b.textContent = k;
      b.addEventListener("click", () => {
        tabsEl.querySelectorAll(".tab").forEach((t) => t.classList.remove("active"));
        b.classList.add("active");
        codeEl.innerHTML = INSTALL[k];
      });
      tabsEl.appendChild(b);
    });
    codeEl.innerHTML = INSTALL[Object.keys(INSTALL)[0]];
  }

  /* ---------- Copy buttons ---------- */
  function setupCopy() {
    document.querySelectorAll("[data-copy]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const sel = btn.getAttribute("data-copy");
        const src = document.querySelector(sel);
        if (!src) return;
        const text = src.textContent;
        try { navigator.clipboard.writeText(text); } catch (e) {}
        const old = btn.textContent;
        btn.textContent = "copied ✓";
        setTimeout(() => { btn.textContent = old; }, 1300);
      });
    });
  }

  /* ---------- Mobile nav ---------- */
  function setupNav() {
    const nav = document.getElementById("nav");
    const toggle = document.getElementById("menu-toggle");
    if (toggle) toggle.addEventListener("click", () => nav.classList.toggle("open"));
    document.querySelectorAll(".nav-links a").forEach((a) => a.addEventListener("click", () => nav.classList.remove("open")));
  }

  /* ---------- Boot ---------- */
  function init() {
    const canvas = document.getElementById("rain-canvas");
    if (canvas && !window.matchMedia("(prefers-reduced-motion: reduce)").matches) startMatrixRain(canvas);
    document.querySelectorAll("[data-decode]").forEach((el, i) => decode(el, 520 + i * 120, 120 + i * 180));
    revealOnScroll();
    setupInstall();
    setupCopy();
    setupNav();
    const yr = document.getElementById("year"); if (yr) yr.textContent = new Date().getFullYear();
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init); else init();
})();
