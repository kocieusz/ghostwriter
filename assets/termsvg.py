"""Render a scripted terminal session as an animated SVG.

Shared by make_demo_svg.py and make_engine_svg.py so both README animations
look like the same tmux session: Tokyo Night colours, a green status bar,
typed commands and instant redraws.

A session is a list of timed changes to a screen of rows. Each row becomes
one <text> element per stretch of time it stays unchanged, switched on and
off with step timing, so it redraws like a terminal instead of fading. Every
row is pinned to the character grid with textLength, so columns line up
whatever monospace font the viewer has.
"""
from html import escape

# Tokyo Night
C = dict(bg="#1a1b26", bar="#16161e", fg="#c0caf5", dim="#565f89", blue="#7aa2f7", cyan="#7dcfff",
         green="#9ece6a", red="#f7768e", yellow="#e0af68", magenta="#bb9af7", border="#3b4261")

CW, LH, FS = 7.8, 18, 13      # cell width, line height, font size
PAD, TOP = 18, 50             # left padding, baseline of the first row
FONT = "ui-monospace,'JetBrains Mono','SF Mono',Menlo,Consolas,'DejaVu Sans Mono',monospace"


def runs(cells, indent="  "):
    """Merge (char, colour) cells into (text, colour) runs."""
    out = [(indent, "fg")]
    for ch, c in cells:
        if len(out) > 1 and out[-1][1] == c:
            out[-1] = (out[-1][0] + ch, c)
        else:
            out.append((ch, c))
    return out


class Session:
    """A terminal of `cols` x `rows` cells, animated over a `loop`-second cycle.

    Rows are addressed as (column, row): a pane is just the column its rows
    start at, so a split layout uses two columns.
    """

    def __init__(self, loop, cols=100, rows=26):
        self.loop, self.cols, self.rows = loop, cols, rows
        self.events = []

    def at(self, t, changes):
        """At time t, set rows: {(col, row): [(text, colour), ...] or None}."""
        self.events.append((t, changes))

    def typed(self, t0, key, cmd, per=0.06):
        """Type `$ cmd` with a block cursor; returns when the line is done."""
        for i in range(len(cmd) + 1):
            self.at(t0 + i * per, {key: [("$ ", "green"), (cmd[:i], "fg"), ("█", "fg")]})
        end = t0 + (len(cmd) + 1) * per + 0.15
        self.at(end, {key: [("$ ", "green"), (cmd, "fg")]})
        return end

    def clear(self, t, col=0, rows=None):
        """Blank every row of the pane starting at `col`."""
        self.at(t, {(col, r): None for r in (rows if rows is not None else range(self.rows))})

    # ------------------------------------------------------------ render
    def render(self, path, *, windows, status_right, title, aria, borders=()):
        """windows: [(start_time, name)] for the tmux status bar."""
        T = self.loop
        width = int(PAD * 2 + self.cols * CW)
        status_y = TOP + self.rows * LH + 6
        height = status_y + 14
        css, body = [], []
        uid = [0]

        def pct(t):
            return f"{max(0.0, min(100.0, t / T * 100)):.3f}%"

        def visible(start, end, at_rest):
            uid[0] += 1
            name = f"k{uid[0]}"
            frames = [f"0%{{opacity:{1 if start <= 0 else 0}}}"]
            if start > 0:
                frames.append(f"{pct(start)}{{opacity:1}}")
            if end < T:
                frames.append(f"{pct(end)}{{opacity:0}}")
            css.append(f"@keyframes {name}{{{''.join(frames)}}}"
                       f".{name}{{opacity:{1 if at_rest else 0};animation:{name} {T}s step-end infinite}}")
            return name

        def text_el(x, y, segs, cls=None, extra=""):
            n = sum(len(t) for t, _ in segs)
            spans = "".join(f'<tspan fill="{C[c]}">{escape(t)}</tspan>' for t, c in segs if t)
            c = f' class="{cls}"' if cls else ""
            return (f'<text{c} x="{x:.1f}" y="{y}" textLength="{n * CW:.1f}" lengthAdjust="spacingAndGlyphs"'
                    f' xml:space="preserve"{extra}>{spans}</text>')

        # each row's history: (start, content) for every change
        screen, timeline = {}, {}
        for t, changes in sorted(self.events, key=lambda e: e[0]):
            for key, val in changes.items():
                if screen.get(key) != val:
                    screen[key] = val
                    timeline.setdefault(key, []).append((t, val))
        for (col, row), changes in timeline.items():
            for i, (start, segs) in enumerate(changes):
                if segs is None:
                    continue
                end = changes[i + 1][0] if i + 1 < len(changes) else T
                cls = visible(start, end, at_rest=(i == len(changes) - 1))
                body.append(text_el(PAD + col * CW, TOP + row * LH, segs, cls))

        # tmux status bar: window list, the current window in reverse video
        bar = [f'<rect x="0" y="{status_y - 14}" width="{width}" height="20" fill="{C["green"]}"/>',
               text_el(PAD, status_y, [("[gw]", "bar")])]
        x = PAD + 5 * CW
        for i, (start, name) in enumerate(windows):
            end = windows[i + 1][0] if i + 1 < len(windows) else T
            word = f"{i}:{name}"
            on = visible(start, end, at_rest=(i == len(windows) - 1))
            bar.append(text_el(x, status_y, [(word, "bar")]))
            bar.append(f'<rect class="{on}" x="{x - CW / 2:.1f}" y="{status_y - 14}" width="{(len(word) + 2) * CW:.1f}" height="20" fill="{C["bar"]}"/>')
            bar.append(text_el(x, status_y, [(word + "*", "green")], on, ' font-weight="bold"'))
            x += (len(word) + 3) * CW
        bar.append(text_el(width - PAD - len(status_right) * CW, status_y, [(status_right, "bar")]))

        lines = "".join(
            f'<line x1="{PAD + b * CW + CW / 2:.1f}" y1="{TOP - 14}" x2="{PAD + b * CW + CW / 2:.1f}" y2="{status_y - 18}" stroke="{C["border"]}"/>'
            for b in borders)
        doc = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}" role="img" aria-label="{escape(aria)}">
<title>{escape(title)}</title>
<style>
text{{font-family:{FONT};font-size:{FS}px;}}
{''.join(css)}
@media (prefers-reduced-motion:reduce){{*{{animation:none!important}}}}
</style>
<rect width="{width}" height="{height}" rx="10" fill="{C['bg']}"/>
<path d="M0 10a10 10 0 0 1 10-10h{width - 20}a10 10 0 0 1 10 10v18H0z" fill="{C['bar']}"/>
<circle cx="18" cy="14" r="5.5" fill="#ff5f57"/><circle cx="36" cy="14" r="5.5" fill="#febc2e"/><circle cx="54" cy="14" r="5.5" fill="#28c840"/>
<text x="{width / 2}" y="18" text-anchor="middle" fill="{C['dim']}" font-size="12">tmux</text>
{lines}
{chr(10).join(body)}
{chr(10).join(bar)}
</svg>
"""
        with open(path, "w", encoding="utf-8") as f:
            f.write(doc)
        return len(doc), uid[0]
