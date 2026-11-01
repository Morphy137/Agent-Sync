import { readFileSync, writeFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import assert from "node:assert/strict";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const requireFromFrontend = createRequire(resolve(root, "frontend/package.json"));
const { Resvg } = requireFromFrontend("@resvg/resvg-js");
const source = readFileSync(resolve(root, "frontend/src/assets/app-icon.png"));
const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024"><clipPath id="corners"><rect width="1024" height="1024" rx="192"/></clipPath><image width="1024" height="1024" clip-path="url(#corners)" href="data:image/png;base64,${source.toString("base64")}"/></svg>`;

function render(size) {
  const image = new Resvg(svg, { fitTo: { mode: "width", value: size } }).render();
  assert.equal(image.width, size);
  assert.equal(image.height, size);
  for (const pixel of [0, size - 1, size * (size - 1), size * size - 1]) {
    assert.ok(image.pixels[pixel * 4 + 3] <= 32);
  }
  assert.equal(image.pixels[(Math.floor(size / 2) * size + Math.floor(size / 2)) * 4 + 3], 255);
  return image.asPng();
}

const sizes = [16, 32, 48, 64, 128, 256];
const images = sizes.map(render);
const header = Buffer.alloc(6 + 16 * images.length);
header.writeUInt16LE(1, 2);
header.writeUInt16LE(images.length, 4);
let offset = header.length;
images.forEach((image, index) => {
  const entry = 6 + index * 16;
  header[entry] = sizes[index] % 256;
  header[entry + 1] = sizes[index] % 256;
  header.writeUInt16LE(1, entry + 4);
  header.writeUInt16LE(32, entry + 6);
  header.writeUInt32LE(image.length, entry + 8);
  header.writeUInt32LE(offset, entry + 12);
  offset += image.length;
});

const ico = Buffer.concat([header, ...images]);
sizes.forEach((size, index) => {
  const entry = 6 + index * 16;
  const start = ico.readUInt32LE(entry + 12);
  assert.equal(ico.readUInt32BE(start + 16), size);
  assert.equal(ico.readUInt32BE(start + 20), size);
  assert.ok(start + ico.readUInt32LE(entry + 8) <= ico.length);
});
const appIcon = render(1024);
writeFileSync(resolve(root, "frontend/src/assets/app-icon-rounded.png"), appIcon);
writeFileSync(resolve(root, "build/appicon.png"), appIcon);
writeFileSync(resolve(root, "build/windows/icon.ico"), ico);
console.log("Generated frontend and build icons");
