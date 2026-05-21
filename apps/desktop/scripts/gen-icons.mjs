/**
 * Generates placeholder icons for Tauri build.
 * Creates solid-color PNG (32x32, 128x128) and ICO files.
 */
import { writeFileSync, mkdirSync } from "fs";
import { deflateSync } from "zlib";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ICONS_DIR = resolve(__dirname, "..", "src-tauri", "icons");
mkdirSync(ICONS_DIR, { recursive: true });

const BRAND_R = 59, BRAND_G = 130, BRAND_B = 246, BRAND_A = 255; // #3B82F6

function crc32(buf) {
  let table = [];
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) crc = table[(crc ^ buf[i]) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

function makePNG(size) {
  const sig = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0);
  ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; ihdr[9] = 6; // 8-bit RGBA
  const ihdrChunk = makeChunk("IHDR", ihdr);

  const raw = [];
  for (let y = 0; y < size; y++) {
    raw.push(0); // filter: none
    for (let x = 0; x < size; x++) {
      raw.push(BRAND_R, BRAND_G, BRAND_B, BRAND_A);
    }
  }
  const compressed = deflateSync(Buffer.from(raw));
  const idatChunk = makeChunk("IDAT", compressed);
  const iendChunk = makeChunk("IEND", Buffer.alloc(0));

  return Buffer.concat([sig, ihdrChunk, idatChunk, iendChunk]);
}

function makeChunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length, 0);
  const typeB = Buffer.from(type, "ascii");
  const crcB = Buffer.alloc(4);
  const crcData = Buffer.concat([typeB, data]);
  crcB.writeUInt32BE(crc32(crcData), 0);
  return Buffer.concat([len, typeB, data, crcB]);
}

function makeICO(pngBuf) {
  const header = Buffer.alloc(6);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(1, 4);

  const entry = Buffer.alloc(16);
  entry[0] = 32; entry[1] = 32;
  entry[2] = 0; entry[3] = 0;
  entry.writeUInt16LE(1, 4);
  entry.writeUInt16LE(32, 6);
  entry.writeUInt32LE(pngBuf.length, 8);
  entry.writeUInt32LE(22, 12);

  return Buffer.concat([header, entry, pngBuf]);
}

const png32 = makePNG(32);
const png128 = makePNG(128);
const ico = makeICO(png32);

writeFileSync(resolve(ICONS_DIR, "32x32.png"), png32);
writeFileSync(resolve(ICONS_DIR, "128x128.png"), png128);
writeFileSync(resolve(ICONS_DIR, "icon.ico"), ico);
console.log("Icons generated:", "32x32.png", "128x128.png", "icon.ico");
