package dashboard

func renderIndexHTML() string {
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Heimdall Dashboard</title>
  <style>
    :root {
      --bg: #f4efe7;
      --panel: #fff9f0;
      --ink: #171717;
      --brand: #0f766e;
      --brand-soft: #99f6e4;
      --warn: #b91c1c;
      --muted: #6b7280;
      --line: #d6d3d1;
      --mono: "Cascadia Code", "Fira Code", ui-monospace, SFMono-Regular, Menlo, monospace;
      --display: "Space Grotesk", "Segoe UI", sans-serif;
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: var(--display);
      color: var(--ink);
      background:
        radial-gradient(circle at 20% 20%, #fef3c7 0%, transparent 45%),
        radial-gradient(circle at 80% 0%, #bae6fd 0%, transparent 35%),
        var(--bg);
      min-height: 100vh;
    }

    .wrap {
      max-width: 1200px;
      margin: 0 auto;
      padding: 24px;
      display: grid;
      gap: 16px;
    }

    .hero {
      border: 1px solid var(--line);
      background: linear-gradient(130deg, var(--panel), #ffffff);
      border-radius: 18px;
      padding: 20px;
      box-shadow: 0 8px 30px rgba(23, 23, 23, 0.08);
      animation: rise 320ms ease-out;
    }

    h1 {
      margin: 0;
      font-size: clamp(1.8rem, 4vw, 2.5rem);
      letter-spacing: 0.03em;
      text-transform: uppercase;
    }

    p {
      margin: 8px 0 0;
      color: var(--muted);
    }

    .tabs {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .tab {
      border: 1px solid var(--line);
      background: #fff;
      border-radius: 999px;
      padding: 8px 12px;
      cursor: pointer;
      font-weight: 600;
      transition: all 160ms ease;
    }

    .tab:hover,
    .tab.active {
      border-color: var(--brand);
      background: var(--brand-soft);
      color: #134e4a;
    }

    .grid {
      display: grid;
      grid-template-columns: repeat(12, 1fr);
      gap: 12px;
    }

    .card {
      grid-column: span 12;
      border: 1px solid var(--line);
      border-radius: 14px;
      background: #fff;
      overflow: hidden;
      animation: rise 280ms ease-out;
    }

    .card h2 {
      margin: 0;
      font-size: 1rem;
      padding: 12px 14px;
      background: #fafaf9;
      border-bottom: 1px solid var(--line);
    }

    .card pre {
      margin: 0;
      padding: 14px;
      max-height: 440px;
      overflow: auto;
      font-family: var(--mono);
      font-size: 0.82rem;
      line-height: 1.5;
      background: #ffffff;
    }

    .two-col .card { grid-column: span 6; }
    .full .card { grid-column: span 12; }

    .note {
      padding: 10px 12px;
      border-radius: 10px;
      border: 1px dashed var(--line);
      color: var(--muted);
      font-size: 0.9rem;
      background: rgba(255,255,255,0.7);
    }

    .err { color: var(--warn); }

    @media (max-width: 900px) {
      .two-col .card { grid-column: span 12; }
      .wrap { padding: 12px; }
    }

    @keyframes rise {
      from { opacity: 0; transform: translateY(10px); }
      to { opacity: 1; transform: translateY(0); }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <h1>Heimdall</h1>
      <p>All-seeing, truth-first observability for Go</p>
    </div>

    <div class="tabs" id="tabs"></div>

    <div class="note" id="hint">Select a menu section to load live API data.</div>

    <div class="grid" id="content"></div>
  </div>

  <script>
    const sections = [
      { key: "overview", label: "Overview", endpoint: "/overview", layout: "full" },
      { key: "explorer", label: "Event Explorer", endpoint: "/events?per_page=25&page=1", layout: "full" },
      { key: "http", label: "HTTP Requests", endpoint: "/events?transport=http&per_page=25&page=1", layout: "full" },
      { key: "grpc", label: "gRPC Requests", endpoint: "/events?transport=grpc&per_page=25&page=1", layout: "full" },
      { key: "errors", label: "Errors", endpoint: "/errors?per_page=25&page=1", layout: "full" },
      { key: "performance", label: "Performance", endpoint: "/performance?per_page=25&page=1", layout: "full" },
      { key: "watchers", label: "Watchers", endpoint: "/watchers", layout: "full" }
    ];

    const tabs = document.getElementById("tabs");
    const content = document.getElementById("content");
    const hint = document.getElementById("hint");

    function setActive(key) {
      for (const button of tabs.querySelectorAll("button")) {
        button.classList.toggle("active", button.dataset.key === key);
      }
    }

    function renderCard(title, data, cls = "") {
      const article = document.createElement("article");
      article.className = "card " + cls;

      const heading = document.createElement("h2");
      heading.textContent = title;

      const pre = document.createElement("pre");
      pre.textContent = JSON.stringify(data, null, 2);

      article.appendChild(heading);
      article.appendChild(pre);
      return article;
    }

    async function loadSection(section) {
      setActive(section.key);
      hint.textContent = "Loading " + section.label + "...";
      content.className = "grid " + section.layout;
      content.innerHTML = "";

      try {
        const response = await fetch(section.endpoint);
        const payload = await response.json();
        if (!response.ok) {
          hint.innerHTML = "<span class=\"err\">API error loading " + section.label + "</span>";
          content.appendChild(renderCard(section.label + " Error", payload));
          return;
        }

        hint.textContent = section.label + " loaded from " + section.endpoint;
        content.appendChild(renderCard(section.label, payload));
      } catch (error) {
        hint.innerHTML = "<span class=\"err\">Request failed for " + section.label + "</span>";
        content.appendChild(renderCard(section.label + " Error", { message: String(error) }));
      }
    }

    for (const section of sections) {
      const button = document.createElement("button");
      button.className = "tab";
      button.dataset.key = section.key;
      button.textContent = section.label;
      button.addEventListener("click", () => loadSection(section));
      tabs.appendChild(button);
    }

    loadSection(sections[0]);
  </script>
</body>
</html>`
}
