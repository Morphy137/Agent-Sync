import { writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const __dirname = dirname(fileURLToPath(import.meta.url));
const requireFromFrontend = createRequire(
  resolve(__dirname, "../frontend/package.json")
);
const { Resvg } = requireFromFrontend("@resvg/resvg-js");

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
  <rect x="0" y="0" width="1024" height="1024" rx="220" fill="#FFFFFF" />
  <g transform="translate(160, 160) scale(29.3333)" fill="none" stroke="#0F0F10" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <rect x="3" y="3" width="11" height="11" rx="2.5" />
    <rect x="10" y="10" width="11" height="11" rx="2.5" />
  </g>
</svg>`;

const resvg = new Resvg(svg, {
  fitTo: {
    mode: "width",
    value: 1024,
  },
});

const pngData = resvg.render();
const pngBuffer = pngData.asPng();
const outPath = resolve(__dirname, "../build/appicon.png");
writeFileSync(outPath, pngBuffer);
console.log(`Generated ${outPath} (${pngData.width}x${pngData.height})`);
