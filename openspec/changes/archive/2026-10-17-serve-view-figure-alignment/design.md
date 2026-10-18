# Design

## Context

`renderStatCombined` in `cmd/spinloop/metrics_render.go` draws the serve
view's one line per series: the label, its gauge half at `serveGaugeW`, the
figure of the current reading, and its bar half at `serveBarW`. The figure
is the only element of the line whose width varies with the data — the
halves are fixed at their constants — and it is drawn with `%.0f%%`, so a
one-digit reading takes two columns, a two-digit three, and 100% four. The
bar half, drawn after the figure, starts at whatever column the figure
left: the sparkline shifts left or right as the reading crosses a digit
boundary.

## Goals / Non-Goals

**Goals:**

- The bar half starts in the same column whatever the figure's digit
  count.
- The line keeps one width across digit counts, so a live view does not
  jitter a series' bar as its reading crosses a boundary.

**Non-Goals:**

- Changing what the figure shows — it stays the reading's percent, and
  keeps the bar's latest sample as its fallback for a stopped engine.
- Touching the gauge, the sparkline, or any other format: they already
  draw the figure after a fixed-width block.
- Changing `serveGaugeW` or `serveBarW`: the halves keep their widths and
  the line keeps fitting the default 80-column window.

## Decisions

- **A fixed-width field of three digits plus the percent sign,
  right-aligned (`%3.0f%%`).** The field is sized to the widest figure a
  percent can take — 100 — not to the common two digits, so no further
  growth is possible and the alignment does not depend on the value range.
  A reading of 7 draws as `  7%`; 42 as ` 42%`; 100 as `100%`.
- **Right-aligned, not left.** A percent is read from its right edge into
  the percent sign; right-alignment keeps the sign in a fixed column, which
  is where the eye lands before the bar.
- **The line gets one column wider for two-digit readings.** The field is
  wider than `%.0f` drew a two-digit figure, so the line's total width
  grows by one column. It still fits the 80-column window the halves are
  sized for, and a stable width is the point: every width is equally
  acceptable, only a shifting one is not.

## Risks / Trade-offs

- None material: the change is a single format verb on one line, and every
  other surface draws the figure after a fixed-width block, so there is no
  second surface to keep in step.
