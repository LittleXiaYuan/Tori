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

// React Aria's shared element transitions call `getAnimations()`, which jsdom
// does not implement. Returning an empty list matches the non-animated test env.
if (typeof (Element.prototype as { getAnimations?: unknown }).getAnimations !== "function") {
  Object.defineProperty(Element.prototype, "getAnimations", {
    configurable: true,
    writable: true,
    value: () => [],
  });
}

// HeroUI Pro components (e.g. sheet's use-scale-background) call
// window.matchMedia, which jsdom does not implement. Provide a minimal
// always-non-matching stub so they can mount in tests.
if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string): MediaQueryList => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList,
  });
}

// jsdom implements neither ResizeObserver nor IntersectionObserver; several
// Pro components (charts, segment indicator, drop-zone) observe layout.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
if (typeof globalThis.IntersectionObserver === "undefined") {
  globalThis.IntersectionObserver = class {
    root = null;
    rootMargin = "";
    thresholds = [];
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() { return []; }
  } as unknown as typeof IntersectionObserver;
}

afterEach(() => {
  cleanup();
});
