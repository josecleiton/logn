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
  const end = document.getElementById("fim");
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
