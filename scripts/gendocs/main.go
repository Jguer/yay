// Command gendocs renders yay's doc/ directory into a static HTML site
// suitable for GitHub Pages.
//
// Usage (from repository root):
//
//	cd scripts && go run ./gendocs [-docs ../doc] [-out ../site]
//
// Pages produced:
//
//	index.html      — landing page
//	man.html        — yay(8) man page (troff → Markdown → HTML)
//	lua.html        — Lua API reference (lua.md → HTML)
//	init-lua.html   — init.lua template as a syntax-highlighted code block
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	bf "github.com/russross/blackfriday/v2"
)

func main() {
	docs := flag.String("docs", "../doc", "documentation source directory")
	out := flag.String("out", "../site", "output directory")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}

	// yay.8 (troff man page) → Markdown → HTML
	man8 := mustRead(filepath.Join(*docs, "yay.8"))
	writePage(*out, "man.html", "yay(8) Manual", mdToHTML(troff2md(man8)))

	// lua.md → HTML
	luaMD := mustRead(filepath.Join(*docs, "lua.md"))
	writePage(*out, "lua.html", "Lua API", mdToHTML(luaMD))

	// init.lua source as a code block
	initLua := mustRead(filepath.Join(*docs, "init.lua"))
	initMD := "# init.lua template\n\n```lua\n" + string(initLua) + "```\n"
	writePage(*out, "init-lua.html", "init.lua template", mdToHTML([]byte(initMD)))

	// index / landing page
	indexMD := mustRead(filepath.Join(*docs, "index.md"))
	writePage(*out, "index.html", "yay", mdToHTML(indexMD))

	fmt.Printf("site written to %s\n", *out)
}

// ── HTML template ─────────────────────────────────────────────────────────────

var pageTmpl = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — yay</title>
<style>
*,*::before,*::after{box-sizing:border-box}
body{font:16px/1.7 system-ui,sans-serif;max-width:820px;margin:0 auto;padding:2rem 1.25rem;color:#1a1a1a}
nav{margin-bottom:2.5rem;padding-bottom:.75rem;border-bottom:1px solid #e0e0e0;display:flex;gap:1.5rem;flex-wrap:wrap}
nav a{text-decoration:none;color:#0067c0;font-size:.95rem}
nav a:hover{text-decoration:underline}
h1{font-size:1.9rem;margin-bottom:.5rem}
h2{font-size:1.2rem;margin:2rem 0 .5rem;padding-bottom:.2rem;border-bottom:1px solid #eee}
h3{font-size:1.05rem;margin:1.5rem 0 .25rem}
p{margin-bottom:.75rem}
ul,ol{margin:0 0 .75rem 1.5rem}
li{margin-bottom:.15rem}
code{font-family:ui-monospace,monospace;font-size:.875em;background:#f4f4f4;padding:1px 5px;border-radius:3px}
pre{background:#f4f4f4;padding:1rem;overflow-x:auto;border-radius:4px;margin-bottom:1rem;line-height:1.5}
pre code{background:none;padding:0;font-size:.85em}
a{color:#0067c0}
strong{font-weight:600}
footer{margin-top:3rem;padding-top:1rem;border-top:1px solid #e0e0e0;color:#666;font-size:.875rem}
</style>
</head>
<body>
<nav>
<a href="index.html">yay</a>
<a href="man.html">Manual</a>
<a href="lua.html">Lua API</a>
<a href="init-lua.html">init.lua</a>
</nav>
<main>
{{.Body}}
</main>
<footer>yay · <a href="https://github.com/Jguer/yay">github.com/Jguer/yay</a></footer>
</body>
</html>`))

type pageData struct {
	Title string
	Body  template.HTML
}

func writePage(dir, name, title string, body []byte) {
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	if err := pageTmpl.Execute(f, pageData{Title: title, Body: template.HTML(body)}); err != nil {
		fatal(err)
	}
	fmt.Printf("  %s\n", path)
}

// ── Markdown → HTML ───────────────────────────────────────────────────────────

func mdToHTML(src []byte) []byte {
	// Build renderer with no Smartypants so that -- stays -- (important for
	// CLI flags like --save). bf.Run's default uses CommonHTMLFlags which
	// enables Smartypants | SmartypantsDashes and would turn -- into &ndash;.
	renderer := bf.NewHTMLRenderer(bf.HTMLRendererParameters{Flags: 0})
	return bf.Run(src,
		bf.WithRenderer(renderer),
		bf.WithExtensions(bf.CommonExtensions|bf.AutoHeadingIDs|bf.DefinitionLists),
	)
}

// ── troff man page → Markdown ─────────────────────────────────────────────────

// troff2md converts the troff/man macros used in yay.8 to Markdown.
// Only macros present in that file are handled; everything else is skipped.
func troff2md(src []byte) []byte {
	var buf bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(src))
	inTP := false // next content is the term of a .TP definition pair

	emit := func(s string) { buf.WriteString(s); buf.WriteByte('\n') }

	for sc.Scan() {
		raw := sc.Text()

		// Blank line: pass through but never emit "**<empty>**" when inTP.
		if strings.TrimSpace(raw) == "" {
			if !inTP {
				emit("")
			}
			continue
		}

		// Plain text (not a macro).
		if !strings.HasPrefix(raw, ".") {
			text := troffInline(raw)
			if inTP {
				emit("**" + text + "**")
				emit("") // blank line so blackfriday keeps term and body as separate <p>s
				inTP = false
			} else {
				emit(text)
			}
			continue
		}

		// Macro line: split on first space to get "MACRO rest".
		macro, rest, _ := strings.Cut(raw[1:], " ")
		rest = strings.TrimSpace(rest)

		switch macro {
		case "TH":
			// .TH "NAME" "SECTION" "DATE" "SOURCE" "MANUAL"
			parts := troffSplitArgs(rest)
			if len(parts) >= 2 {
				emit("# " + parts[0] + "(" + parts[1] + ")")
				emit("")
			}
		case "SH":
			emit("")
			emit("## " + troffInline(rest))
			emit("")
			inTP = false
		case "SS":
			emit("")
			emit("### " + troffInline(rest))
			emit("")
		case "TP":
			emit("")
			inTP = true
		case "B":
			if rest == "" {
				break
			}
			emit("**" + troffInline(rest) + "**")
			if inTP {
				emit("")
				inTP = false
			}
		case "I":
			if rest == "" {
				break
			}
			emit("_" + troffInline(rest) + "_")
			if inTP {
				emit("")
				inTP = false
			}
		case "BR":
			emit(troffAlternating(rest, true))
			inTP = false
		case "IR":
			emit(troffAlternating(rest, false))
			inTP = false
		case "RE", "RS", "PP", "LP", "P", "sp", "br":
			emit("")
			// .nh .ad .nf .fi .in .ta and anything else: silently skip.
		}
	}

	return buf.Bytes()
}

// troffInline converts troff inline escapes to Markdown equivalents.
//
//	\fB ... \fR  →  **...**
//	\fI ... \fR  →  _..._
//	\-           →  -
//	\%           →  (removed — optional hyphenation point)
//	\\           →  \
//	\<other>     →  <other>  (pass-through)
func troffInline(s string) string {
	var b strings.Builder
	cur := byte('R') // current font: R=roman B=bold I=italic

	openFont := func(f byte) {
		switch f {
		case 'B':
			b.WriteString("**")
		case 'I':
			b.WriteByte('_')
		}
	}
	closeFont := func(f byte) {
		switch f {
		case 'B':
			b.WriteString("**")
		case 'I':
			b.WriteByte('_')
		}
	}

	for i := 0; i < len(s); {
		if s[i] != '\\' {
			// Angle brackets are troff placeholder delimiters (e.g. <dir>),
			// not HTML. Encode them so blackfriday does not treat them as
			// inline HTML and produce invalid nesting.
			switch s[i] {
			case '<':
				b.WriteString("&lt;")
			case '>':
				b.WriteString("&gt;")
			default:
				b.WriteByte(s[i])
			}
			i++
			continue
		}
		if i+1 >= len(s) {
			// trailing backslash — skip
			i++
			continue
		}
		switch s[i+1] {
		case 'f':
			// font change: \fX where X is B, I, R, P, or a digit
			closeFont(cur)
			if i+2 < len(s) {
				switch s[i+2] {
				case 'B':
					cur = 'B'
					openFont('B')
				case 'I':
					cur = 'I'
					openFont('I')
				default: // R, P, or any other reset
					cur = 'R'
				}
				i += 3
			} else {
				cur = 'R'
				i += 2
			}
		case '-':
			b.WriteByte('-')
			i += 2
		case '%':
			i += 2 // optional break point — discard
		case '\\':
			b.WriteByte('\\')
			i += 2
		default:
			b.WriteByte(s[i+1])
			i += 2
		}
	}
	closeFont(cur)
	return b.String()
}

// troffSplitArgs splits a troff argument string into tokens.
// Tokens may be double-quoted ("foo bar") or plain words separated by whitespace.
// Each token is passed through troffInline to resolve escape sequences.
func troffSplitArgs(s string) []string {
	var out []string
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		if s[0] == '"' {
			end := strings.IndexByte(s[1:], '"')
			if end < 0 {
				out = append(out, troffInline(s[1:]))
				break
			}
			out = append(out, troffInline(s[1:end+1]))
			s = strings.TrimSpace(s[end+2:])
		} else {
			i := strings.IndexAny(s, " \t")
			if i < 0 {
				out = append(out, troffInline(s))
				break
			}
			out = append(out, troffInline(s[:i]))
			s = strings.TrimSpace(s[i:])
		}
	}
	return out
}

// troffAlternating renders .BR / .IR args by alternating bold (or italic)
// and roman styles, concatenated without spaces (the troff convention).
func troffAlternating(args string, startBold bool) string {
	parts := troffSplitArgs(args)
	var b strings.Builder
	for i, p := range parts {
		if (i%2 == 0) == startBold {
			b.WriteString("**")
			b.WriteString(p)
			b.WriteString("**")
		} else {
			b.WriteString(p)
		}
	}
	return b.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	return data
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gendocs:", err)
	os.Exit(1)
}

