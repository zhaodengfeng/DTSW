package fallback

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func RenderCaddyfile(cfg config.Config) string {
	return strings.TrimSpace(fmt.Sprintf(`
{
	auto_https off
	admin off
}

http://%s {
	root * %s
	encode zstd gzip
	header {
		-Server
		Referrer-Policy "strict-origin-when-cross-origin"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "SAMEORIGIN"
	}
	file_server
}
`, cfg.Fallback.ListenAddress, cfg.Fallback.SiteRoot)) + "\n"
}

func DefaultSiteFiles(cfg config.Config) map[string][]byte {
	siteName := siteDisplayName(cfg.Server.Domain)
	index := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <meta name="description" content="%s is a quiet place for field notes, travel letters, and short essays.">
  <link rel="stylesheet" href="/styles.css">
</head>
<body>
  <div class="page-shell">
    <header class="masthead">
      <p class="eyebrow">Independent Journal</p>
      <h1>%s</h1>
      <p class="lede">A small collection of notes on travel, design, and the habits that help good work last.</p>
    </header>

    <main class="grid">
      <article class="feature-card">
        <p class="section-label">Latest dispatch</p>
        <h2>The Discipline Of Leaving Things Simple</h2>
        <p>Simple systems are not naive. They are usually the result of saying no to clutter often enough that the useful parts can keep breathing.</p>
        <a href="/journal/" class="link-pill">Read the journal</a>
      </article>

      <aside class="side-card">
        <p class="section-label">About this site</p>
        <p>This site is intentionally light: fast to load, pleasant to read, and calm enough to stay out of the way.</p>
        <a href="/about/" class="link-pill secondary">Learn more</a>
      </aside>
    </main>
  </div>
</body>
</html>
`, siteName, siteName, siteName)

	journal := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Journal | %s</title>
  <link rel="stylesheet" href="/styles.css">
</head>
<body>
  <div class="page-shell page-inner">
    <p class="eyebrow"><a href="/">Home</a></p>
    <h1>Journal</h1>
    <div class="entry-list">
      <article>
        <p class="entry-date">April 2026</p>
        <h2>What A Quiet Website Still Does Well</h2>
        <p>It loads quickly, respects attention, and makes room for the writing rather than decorating every edge of the page.</p>
      </article>
      <article>
        <p class="entry-date">March 2026</p>
        <h2>Three Notes On Sustainable Tools</h2>
        <p>Choose defaults that are easy to repair, prefer a shorter chain of moving parts, and leave enough headroom for boring maintenance.</p>
      </article>
    </div>
  </div>
</body>
</html>
`, siteName)

	about := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>About | %s</title>
  <link rel="stylesheet" href="/styles.css">
</head>
<body>
  <div class="page-shell page-inner">
    <p class="eyebrow"><a href="/">Home</a></p>
    <h1>About</h1>
    <p>%s is a small editorial-style website generated automatically by DTSW to provide a stable, normal-looking fallback experience.</p>
    <p>The content is intentionally simple and static so the site stays fast, easy to serve, and easy to repair.</p>
  </div>
</body>
</html>
`, siteName, siteName)

	styles := `:root {
  color-scheme: light;
  --paper: #f6f1e8;
  --paper-strong: #efe4d1;
  --ink: #171717;
  --muted: #6a6258;
  --accent: #8f4d2e;
  --border: rgba(23, 23, 23, 0.14);
}

* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif;
  color: var(--ink);
  background:
    radial-gradient(circle at top, rgba(143, 77, 46, 0.08), transparent 42%),
    linear-gradient(160deg, #f8f4ec, #fbfaf7 48%, #f1e7d6);
}

a { color: inherit; }
.page-shell {
  width: min(1040px, calc(100vw - 32px));
  margin: 0 auto;
  padding: 72px 0 96px;
}

.masthead {
  margin-bottom: 48px;
  padding: 40px;
  border: 1px solid var(--border);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 22px 50px rgba(0, 0, 0, 0.06);
}

.eyebrow, .section-label, .entry-date {
  margin: 0 0 12px;
  font-family: "Avenir Next", "Helvetica Neue", Helvetica, Arial, sans-serif;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-size: 0.78rem;
  color: var(--muted);
}

h1, h2 {
  margin: 0 0 16px;
  font-weight: 600;
  line-height: 1.06;
}

h1 { font-size: clamp(2.8rem, 6vw, 5.3rem); }
h2 { font-size: clamp(1.6rem, 3vw, 2.2rem); }

.lede, p {
  margin: 0;
  max-width: 64ch;
  font-size: 1.08rem;
  line-height: 1.8;
  color: #2e2a25;
}

.grid {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(280px, 1fr);
  gap: 24px;
}

.feature-card, .side-card, .page-inner, .entry-list article {
  padding: 32px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 18px 36px rgba(0, 0, 0, 0.05);
}

.side-card { background: rgba(239, 228, 209, 0.52); }
.page-inner { max-width: 760px; }
.entry-list { display: grid; gap: 20px; }

.link-pill {
  display: inline-flex;
  align-items: center;
  margin-top: 24px;
  padding: 12px 18px;
  border-radius: 999px;
  background: var(--accent);
  color: white;
  text-decoration: none;
  font-family: "Avenir Next", "Helvetica Neue", Helvetica, Arial, sans-serif;
  font-weight: 600;
}

.link-pill.secondary {
  background: transparent;
  color: var(--accent);
  border: 1px solid rgba(143, 77, 46, 0.26);
}

@media (max-width: 820px) {
  .page-shell { width: min(100vw - 24px, 740px); padding: 28px 0 48px; }
  .masthead, .feature-card, .side-card, .page-inner, .entry-list article { padding: 24px; border-radius: 20px; }
  .grid { grid-template-columns: 1fr; }
}
`

	return map[string][]byte{
		"index.html":                           []byte(index),
		filepath.Join("journal", "index.html"): []byte(journal),
		filepath.Join("about", "index.html"):   []byte(about),
		"styles.css":                           []byte(styles),
		"robots.txt":                           []byte("User-agent: *\nAllow: /\n"),
	}
}

func siteDisplayName(domain string) string {
	name := strings.TrimSpace(domain)
	if name == "" {
		return "Field Notes"
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		name = parts[0]
	}
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return titleWords(name)
}

func titleWords(value string) string {
	parts := strings.Fields(value)
	for i, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		for j := 1; j < len(runes); j++ {
			runes[j] = []rune(strings.ToLower(string(runes[j])))[0]
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
