import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// jsdom's Blob has no `stream()` method, but Node's global Response (undici)
// calls `blob.stream()` when a Blob/File is passed as a body — e.g.
// `new Response(new Blob([...]))` in tests — and throws
// "object.stream is not a function" without it. Polyfill a WHATWG
// ReadableStream backed by the blob's bytes when the runtime omits one.
if (typeof (Blob.prototype as { stream?: unknown }).stream !== "function") {
  Object.defineProperty(Blob.prototype, "stream", {
    configurable: true,
    writable: true,
    value(this: Blob): ReadableStream<Uint8Array> {
      const blob = this;
      return new ReadableStream<Uint8Array>({
        async start(controller) {
          controller.enqueue(new Uint8Array(await blob.arrayBuffer()));
          controller.close();
        },
      });
    },
  });
}

afterEach(() => {
  cleanup();
});
