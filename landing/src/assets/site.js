// Revoada de balões quando o fim da página aparece — uma vez por visitante.
//
// Tudo por CSSOM (`style.setProperty`) e `createElementNS`: a CSP não aceita estilo
// inline nem `innerHTML`, e isto não precisa de nenhum dos dois.
(() => {
  const KEY = "logn-landing-celebrated";
  const COLORS = ["#E4572E", "#F5C451", "#3DB2FF", "#6BCB77", "#C77DFF", "#FF6FB5", "#4ECDC4",
    "#F4A261", "#9BC53D", "#D64550", "#7C8BFF", "#D8DEE4", "#00B894", "#FF7A45"];
  const COUNT = 64;
  const LIFETIME_MS = 13500;
  const SVG = "http://www.w3.org/2000/svg";

  const party = document.querySelector(".party");
  const end = document.getElementById("end");
  if (!party || !end || !("IntersectionObserver" in window)) return;

  const seen = () => { try { return localStorage.getItem(KEY) === "1"; } catch (e) { return false; } };
  const remember = () => { try { localStorage.setItem(KEY, "1"); } catch (e) { /* aba privada */ } };
  const reduced = () => window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  const path = (d, attrs) => {
    const p = document.createElementNS(SVG, "path");
    p.setAttribute("d", d);
    for (const k in attrs) p.setAttribute(k, attrs[k]);
    return p;
  };

  const balloon = () => {
    const color = COLORS[Math.floor(Math.random() * COLORS.length)];
    const size = 26 + Math.pow(Math.random(), 1.6) * 60;
    const delay = Math.pow(Math.random(), 1.4) * 4;
    const tilt = (Math.random() * 10 + 4) * (Math.random() > 0.5 ? 1 : -1);

    const rise = document.createElement("div");
    rise.className = "rise";
    rise.style.setProperty("left", Math.random() * 96 + "%");
    rise.style.setProperty("bottom", -size * 1.6 + "px");
    rise.style.setProperty("width", size + "px");
    rise.style.setProperty("--dx", (Math.random() - 0.5) * 160 + "px");
    rise.style.setProperty("--dur", 4.2 + Math.random() * 4.2 + "s");
    rise.style.setProperty("--delay", delay + "s");

    const sway = document.createElement("div");
    sway.className = "sway";
    sway.style.setProperty("--r0", -tilt + "deg");
    sway.style.setProperty("--r1", tilt + "deg");
    sway.style.setProperty("--sway", 1.6 + Math.random() + "s");
    sway.style.setProperty("--delay", delay + "s");

    const svg = document.createElementNS(SVG, "svg");
    svg.setAttribute("viewBox", "0 0 96 122");
    svg.setAttribute("width", size);
    svg.setAttribute("height", size * 122 / 96);
    svg.append(
      path("M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z", { fill: color }),
      path("M31 21 C27 27 25 33 25 40", { class: "hl" }),
      path("M41 76 L55 76 L48 91 Z", { fill: color }),
      path("M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116", { class: "string", stroke: color }),
    );
    sway.append(svg);
    rise.append(sway);
    return rise;
  };

  let timer = 0;
  const observer = new IntersectionObserver((entries) => {
    if (!entries.some((e) => e.isIntersecting)) return;
    observer.disconnect();
    if (seen() || reduced()) return;
    remember();
    for (let i = 0; i < COUNT; i++) party.append(balloon());
    clearTimeout(timer);
    timer = setTimeout(() => party.replaceChildren(), LIFETIME_MS);
  }, { threshold: 0.6 });
  observer.observe(end);
})();

// iPhone e iPad: o app ainda não está na App Store, então os botões de baixar levam à
// lista de espera do fim da página, com o e-mail já focado. Só enquanto o formulário
// existir — com a App Store no ar, o build o tira e tudo volta ao link da loja. Devolve
// o que desvia um botão, que o teste do celular usa no do veredito; fora do iPhone, `null`.
const toWaitlist = (() => {
  const email = document.getElementById("waitlist-email");
  // O iPad se apresenta como Mac; o que o entrega é a tela de toque.
  const apple = /iPhone|iPad|iPod/.test(navigator.userAgent)
    || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  if (!email || !apple) return null;

  const reduced = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const scroll = (e) => {
    e.preventDefault();
    email.form.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "center" });
    email.focus({ preventScroll: true });
  };
  // O `href` muda junto, para o "abrir link" do toque longo não cair no Google Play.
  const go = (el) => {
    el.setAttribute("href", "#end");
    el.addEventListener("click", scroll);
  };
  // O selo do Google Play fica, para mostrar que o app existe; o botão "Me avise" entra
  // ao lado dele. Só o botão do topo, que é texto, troca de texto e de destino.
  for (const el of document.querySelectorAll('[data-shop="play"][data-ios]')) {
    el.textContent = el.dataset.ios;
    go(el);
  }
  for (const el of document.querySelectorAll('[data-shop="waitlist"]')) {
    el.hidden = false;
    go(el);
  }
  return go;
})();

// O celular do topo vira o teste: arrastar (ou tocar) um bloco, confirmar e receber AC,
// WA ou TLE, como no app. O julgamento é local — a CSP não deixa a página falar com
// ninguém — e o problema é inventado (AGENTS.md, regra 8). Sem JS, fica a imagem.
(() => {
  const phone = document.querySelector(".phone");
  if (!phone) return;

  const LIMIT_MS = 20000;  // o relógio da questão; zerou, é TLE
  const LOW_S = 5;         // o relógio fica vermelho nos últimos segundos
  const START_LIVES = 2;   // o mock começa com uma vida perdida
  const SLOP = 6;          // px até um toque virar arrasto

  const $ = (sel) => phone.querySelector(sel);
  const d = phone.dataset;
  const screen = $(".phone-screen");
  const bar = $(".ph-bar");
  const clock = $(".ph-clock");
  const livesBox = $(".ph-lives");
  const hearts = [...livesBox.children];
  const letters = $(".ph-letters");
  const letter = $("[data-letter]");
  const problem = $("[data-problem]");
  const slot = $("[data-slot]");
  const drop = $("[data-drop]");
  const dropEmpty = drop.textContent;
  const chips = [...phone.querySelectorAll(".ph-chip")];
  const confirm = $("[data-confirm]");
  const verdict = $("[data-verdict]");
  const stageOk = $("[data-stage-ok]");
  const stageErr = $("[data-stage-err]");
  const code = $("[data-code]");
  const meaning = $("[data-meaning]");
  const trap = $("[data-trap]");
  const trapCat = $("[data-trap-cat]");
  const trapJudge = $("[data-trap-judge]");
  const trapBody = $("[data-trap-body]");
  const next = $("[data-continue]");
  const store = $("[data-store]");
  const restart = $("[data-restart]");
  const live = $("[data-live]");

  const reduced = () => window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const fill = (tpl, key, value) => tpl.replace(`%{${key}}`, value);
  const focus = (el) => el.focus({ preventScroll: true });

  let lives = START_LIVES;
  let placed = null;        // o bloco na lacuna
  let left = LIMIT_MS;
  let started = false;      // o relógio só anda depois do primeiro toque
  let judging = false;      // veredito na tela: relógio parado
  let onScreen = true;      // celular fora da tela: relógio parado
  let timer = 0;
  let last = 0;

  // --- Relógio --------------------------------------------------------------

  function paintClock() {
    const s = Math.ceil(left / 1000);
    clock.textContent = `00:${String(s).padStart(2, "0")}`;
    clock.classList.toggle("is-low", started && s <= LOW_S);
  }

  const running = () => started && !judging && onScreen && !document.hidden;

  function tick() {
    const now = performance.now();
    if (running()) {
      left = Math.max(0, left - (now - last));
      paintClock();
      if (left === 0) judge("TLE");
    }
    last = now;
  }

  // Liga ou desliga o intervalo conforme o estado. Pausado, o tempo não corre.
  function sync() {
    if (running() && !timer) {
      last = performance.now();
      timer = setInterval(tick, 200);
    } else if (!running() && timer) {
      clearInterval(timer);
      timer = 0;
    }
  }

  function start() {
    if (started) return;
    started = true;
    sync();
  }

  // --- Vidas ----------------------------------------------------------------

  function paintLives() {
    hearts.forEach((h, i) => h.classList.toggle("ph-lost", i >= lives));
    const label = [d.livesZero, d.livesOne, d.livesTwo][lives];
    livesBox.title = label;
    livesBox.setAttribute("aria-label", label);
  }

  // --- Lacuna ---------------------------------------------------------------

  function place(chip) {
    if (judging) return;
    start();
    if (placed) placed.classList.remove("is-placed");
    placed = chip;
    chip.classList.add("is-placed");
    const text = chip.textContent;
    slot.textContent = text;
    slot.classList.add("is-filled");
    drop.textContent = text;
    drop.classList.add("is-filled");
    drop.setAttribute("aria-label", fill(d.dropFilled, "chip", text));
    drop.disabled = false;
    confirm.disabled = false;
    // O bloco some do banco, e o foco iria junto.
    focus(confirm);
  }

  function unplace() {
    const chip = placed;
    if (!chip) return null;
    placed = null;
    chip.classList.remove("is-placed");
    slot.textContent = "";
    slot.classList.remove("is-filled");
    drop.textContent = dropEmpty;
    drop.classList.remove("is-filled");
    drop.removeAttribute("aria-label");
    drop.disabled = true;
    confirm.disabled = true;
    return chip;
  }

  // --- Veredito -------------------------------------------------------------

  function shake() {
    bar.classList.remove("is-shaking", "is-flashing");
    void bar.offsetWidth;  // reinicia a animação
    bar.classList.add(reduced() ? "is-flashing" : "is-shaking");
  }
  bar.addEventListener("animationend", () => bar.classList.remove("is-shaking", "is-flashing"));

  function judge(kind, out) {
    judging = true;
    sync();
    const ok = kind === "AC";
    if (!ok) lives -= 1;
    const over = !ok && lives === 0;

    problem.hidden = true;
    verdict.hidden = false;
    stageOk.hidden = !ok;
    stageErr.hidden = ok;
    // Como no app: a fileira de balões só aparece no acerto, com o F cheio.
    letters.hidden = !ok;
    letter.classList.toggle("b-done", ok);
    letter.classList.toggle("b-current", !ok);
    trap.hidden = ok;

    if (!ok) {
      const tle = kind === "TLE";
      code.textContent = kind;
      meaning.textContent = tle ? meaning.dataset.tle : meaning.dataset.wa;
      trapCat.textContent = tle ? d.tleCat : d.waCat;
      trapJudge.textContent = tle ? d.tleJudge : fill(d.waJudge, "out", out);
      trapBody.textContent = tle ? d.tleBody : "";
      trapBody.hidden = !tle;
      paintLives();
      shake();
    }

    // Erro com vida sobrando volta ao problema; acerto ou fim das vidas leva ao app.
    next.hidden = ok || over;
    store.hidden = !(ok || over);
    store.textContent = toWaitlist ? store.dataset.ios : ok ? store.dataset.ac : store.dataset.over;
    restart.hidden = !(ok || over);

    const said = (ok ? stageOk : stageErr).textContent + " " + (ok ? "" : trap.textContent);
    live.textContent = said.replace(/\s+/g, " ").trim();
    focus(next.hidden ? store : next);
  }

  function resetProblem() {
    unplace();
    judging = false;
    started = false;
    left = LIMIT_MS;
    paintClock();
    verdict.hidden = true;
    problem.hidden = false;
    letters.hidden = false;
    live.textContent = "";
    sync();
    focus(chips[0]);
  }

  function resetSession() {
    lives = START_LIVES;
    paintLives();
    letter.classList.remove("b-done");
    letter.classList.add("b-current");
    resetProblem();
  }

  // --- Toque, teclado e arrasto ---------------------------------------------

  // O clique cobre toque, mouse e Enter/Espaço. Depois de um arrasto, o navegador ainda
  // manda um clique no bloco; esse é engolido.
  let swallowClick = false;
  let drag = null;

  const overTarget = (x, y) => [drop, slot].some((el) => {
    const r = el.getBoundingClientRect();
    return x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
  });

  function endDrag(e, dropIt) {
    const current = drag;
    drag = null;
    if (!current || !current.ghost) return;
    current.ghost.remove();
    current.chip.classList.remove("is-lifted");
    drop.classList.remove("is-over");
    swallowClick = true;
    setTimeout(() => { swallowClick = false; });
    if (dropIt && overTarget(e.clientX, e.clientY)) place(current.chip);
  }

  for (const chip of chips) {
    chip.addEventListener("click", () => {
      if (swallowClick) return;
      place(chip);
    });
    chip.addEventListener("pointerdown", (e) => {
      if (judging || e.button !== 0) return;
      e.preventDefault();  // sem seleção de texto nem arrasto nativo
      drag = { chip, id: e.pointerId, x0: e.clientX, y0: e.clientY, ghost: null, ox: 0, oy: 0 };
      chip.setPointerCapture(e.pointerId);
    });
    chip.addEventListener("pointermove", (e) => {
      if (!drag || drag.id !== e.pointerId) return;
      if (!drag.ghost) {
        if (Math.hypot(e.clientX - drag.x0, e.clientY - drag.y0) < SLOP) return;
        start();
        const r = chip.getBoundingClientRect();
        drag.ox = drag.x0 - r.left;
        drag.oy = drag.y0 - r.top;
        drag.ghost = document.createElement("div");
        drag.ghost.className = "ph-ghost";
        drag.ghost.setAttribute("aria-hidden", "true");
        drag.ghost.textContent = chip.textContent;
        document.body.append(drag.ghost);
        chip.classList.add("is-lifted");
      }
      drag.ghost.style.setProperty("left", e.clientX - drag.ox + "px");
      drag.ghost.style.setProperty("top", e.clientY - drag.oy + "px");
      drop.classList.toggle("is-over", overTarget(e.clientX, e.clientY));
    });
    chip.addEventListener("pointerup", (e) => endDrag(e, true));
    chip.addEventListener("pointercancel", (e) => endDrag(e, false));
  }

  drop.addEventListener("click", () => {
    if (judging) return;
    const chip = unplace();
    if (chip) focus(chip);
  });
  confirm.addEventListener("click", () => {
    if (!placed || judging) return;
    judge("ok" in placed.dataset ? "AC" : "WA", placed.dataset.out);
  });
  if (toWaitlist) toWaitlist(store);
  next.addEventListener("click", resetProblem);
  restart.addEventListener("click", resetSession);

  // Celular fora da tela ou aba escondida: o relógio espera.
  if ("IntersectionObserver" in window) {
    new IntersectionObserver((entries) => {
      onScreen = entries[entries.length - 1].intersectionRatio >= 0.2;
      sync();
    }, { threshold: [0, 0.2] }).observe(phone);
  }
  document.addEventListener("visibilitychange", sync);

  // Acende: de imagem para teste.
  phone.setAttribute("role", "group");
  phone.setAttribute("aria-label", d.playLabel);
  screen.removeAttribute("aria-hidden");
  livesBox.setAttribute("role", "img");
  for (const chip of chips) chip.disabled = false;
  paintLives();
  paintClock();
})();
