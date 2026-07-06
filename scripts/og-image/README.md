# OG Image Generator

Generates the site's Open Graph image (`docs/public/og-image.png`, 2400x1260 =
1200x630 @2x) from an HTML template rendered with headless Chrome.

The template (`og-image.html`) mirrors the landing hero's visual language
(`docs/.vitepress/components/landing/sections/HeroDemo.vue` and the ambient
backdrop in `LandingPageNew.vue`): black background, teal `#4fd1cb` accent,
the 46px cell grid with one frame of the left-to-right teal sweep frozen in
place, the "Infrastructure as Data" badge, and the hero headline. Cell
placement uses a seeded PRNG, so renders are deterministic.

## Usage

```bash
# Regenerate the production OG image
./scripts/og-image/render.sh

# Render a draft to inspect first
./scripts/og-image/render.sh /tmp/og-draft.png
```

Requires Chrome or Chromium (autodetected; override with
`CHROME=/path/to/chrome`).

## Updating

Edit `og-image.html` (copy, colors, sweep band position/intensity, resource
chips) and re-run the script. Keep it in sync with the landing hero when the
brand style changes. The logo is referenced from `docs/public/logo.png` via a
relative path, so it stays current automatically.
