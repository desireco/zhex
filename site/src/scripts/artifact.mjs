// Shared compiled-artifact loading (#113): the run panels and the Life demo
// fetch the same wasm bytes through one throwing helper with one cache, so
// each artifact is fetched at most once per page no matter how many consumers
// ask. fetchArtifact owns the whole degradation ladder — a browser without
// WebAssembly GC and an artifact the browser cannot validate throw
// oldBrowserMessage; a failed fetch or HTTP status throws loadFailedMessage.

const oldBrowserMessage = "This browser can't run Zhex programs — it lacks WebAssembly garbage collection. Zhex needs roughly Chrome 119+, Firefox 120+, or Safari 18.2+.";
export const loadFailedMessage = "The compiled program could not be loaded. Check your connection and reload the page to try again.";

// Cached promises, not bytes: two consumers asking at once share one fetch,
// and eviction on rejection keeps a failed load from poisoning the cache —
// the next call re-attempts it.
const artifactCache = new Map();

async function loadArtifact(url) {
  let response;
  try {
    response = await fetch(url);
  } catch {
    throw new Error(loadFailedMessage);
  }
  if (!response.ok) throw new Error(loadFailedMessage);
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (WebAssembly.validate(bytes) === false) throw new Error(oldBrowserMessage);
  return bytes;
}

export function fetchArtifact(url) {
  if (typeof WebAssembly === "undefined") return Promise.reject(new Error(oldBrowserMessage));
  let pending = artifactCache.get(url);
  if (pending === undefined) {
    pending = loadArtifact(url);
    pending.catch(() => artifactCache.delete(url));
    artifactCache.set(url, pending);
  }
  return pending;
}
