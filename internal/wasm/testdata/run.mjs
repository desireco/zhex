// Differential harness runner: the browser host.
// node run.mjs <module.wasm> [--root <dir>] [--stdin <file>] [--env K=V ...]
//
// Normative for the import and trap contract in internal/wasm/DESIGN.md
// (§4 imports, §6 runtime diagnostics), the milestone-2 fail-code extension
// (DESIGN-M2 §2c), and the milestone-3 host surface (DESIGN-M3 §1a imports,
// §2 virtual FS, §4 stdin): 15 env.* imports, an in-memory file tree seeded
// from --root, a stdin string from --stdin, an env table from repeated
// --env, and the [u32 len][bytes] scratch protocol at memory offset 0 with
// host-side memory growth. Only linear memory crosses the boundary.
//
// env.print sends raw UTF-8 stdout bytes (no newline added — the prelude's
// print helper appends "\n" itself; DESIGN-M3 Surprises 2).
import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";

const failMessages = {
  1: "integer overflow",
  2: "word call depth limit of 256 exceeded",
  3: "list index is out of range",
  4: "array coordinate is out of range",
  5: "array rank must be between 2 and 32",
  6: "array extents cannot be negative",
  7: "array index has the wrong rank",
  8: "list exceeds element limit",
  9: "array exceeds element limit",
  10: "collection is being iterated",
  11: "unhandled failure",
  13: "text.replace old text is empty",
  14: "text.split exceeds element limit",
};

// Dynamic-message codes stage their message as UTF-8 at memory offset 0;
// $zhex_fail_msg records the byte length for zhex_msg_len() (DESIGN-M2 §2c;
// codes 16-18 join in M3 — top-level unhandled Result, file.stream loop
// failures, plain file.exists stat failure).
const stagedFailCodes = new Set([12, 15, 16, 17, 18]);

// WASI preview 1 errno numbering (DESIGN-M3 §1a); the prelude owns the
// errno-to-text table, the host only returns numbers.
const ERRNO = {
  SUCCESS: 0,
  BADF: 8,
  EACCES: 2,
  EEXIST: 20,
  EINVAL: 28,
  EISDIR: 31,
  ENOENT: 44,
  ENOTDIR: 54,
  ENOTEMPTY: 55,
};

const argv = process.argv.slice(2);
const wasmPath = argv.shift();
let rootDir = null;
let stdinPath = null;
const envTable = new Map();
while (argv.length > 0) {
  const arg = argv.shift();
  if (arg === "--root") rootDir = argv.shift();
  else if (arg === "--stdin") stdinPath = argv.shift();
  else if (arg === "--env") {
    const pair = argv.shift();
    const eq = pair.indexOf("=");
    envTable.set(pair.slice(0, eq), pair.slice(eq + 1));
  } else {
    console.error(`run.mjs: unknown argument ${arg}`);
    process.exit(2);
  }
}

// The virtual FS: { kind: "dir", entries: Map<string, node> } or
// { kind: "file", data: Uint8Array }. Paths resolve verbatim against the
// root with "/" separators (DESIGN-M3 §2); no permissions are modeled.
const decoder = new TextDecoder();

async function seedDir(node, hostPath) {
  for (const entry of await readdir(hostPath, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      const child = { kind: "dir", entries: new Map() };
      node.entries.set(entry.name, child);
      await seedDir(child, join(hostPath, entry.name));
    } else if (entry.isFile()) {
      node.entries.set(entry.name, { kind: "file", data: await readFile(join(hostPath, entry.name)) });
    }
  }
}
const fsRoot = { kind: "dir", entries: new Map() };
if (rootDir !== null) await seedDir(fsRoot, rootDir);

const stdinBytes = stdinPath !== null ? await readFile(stdinPath) : new Uint8Array(0);
let stdinOffset = 0;

const bytes = await readFile(wasmPath);
const chunks = [];
let memory = null;

const mem = () => new Uint8Array(memory.buffer);
const view = () => new DataView(memory.buffer);

const readBytes = (ptr, len) => mem().subarray(ptr, ptr + len);
const readText = (ptr, len) => decoder.decode(readBytes(ptr, len));

// scratch writes host-to-module data at memory offset 0 as [u32 byte
// length][bytes], growing memory first (DESIGN-M3 §1a scratch protocol); the
// module consumes it synchronously, so the slot is reusable per call.
function scratch(data) {
  const need = 4 + data.length;
  const pages = Math.ceil(need / 65536);
  if (memory.buffer.byteLength < need) memory.grow(pages);
  view().setUint32(0, data.length, true);
  mem().set(data, 4);
}

const scratchBytes = (data) => scratch(typeof data === "string" ? new TextEncoder().encode(data) : data);

// Path resolution: split on "/", drop empty and "." segments; ".." is never
// produced by the fixture discipline (DESIGN-M3 §2 portable paths) and is
// left to miss rather than resolved.
function segments(path) {
  return path.split("/").filter((part) => part !== "" && part !== ".");
}
function lookup(path) {
  let node = fsRoot;
  for (const part of segments(path)) {
    if (node.kind !== "dir") return null;
    node = node.entries.get(part) ?? null;
    if (node === null) return null;
  }
  return node;
}
function parentOf(path) {
  const parts = segments(path);
  const name = parts.pop();
  if (name === undefined) return null; // the root itself
  let node = fsRoot;
  for (const part of parts) {
    if (node.kind !== "dir") return null;
    node = node.entries.get(part) ?? null;
    if (node === null) return null;
  }
  return { dir: node, name };
}

// Open handles read live node contents with a byte offset (DESIGN-M3 §2).
const handles = [];
let handleSeq = 0;

const host = {
  print(ptr, len) {
    chunks.push(Uint8Array.from(readBytes(ptr, len)));
  },
  stdin_read(ptr, cap) {
    if (stdinOffset >= stdinBytes.length) return 0;
    const n = Math.min(cap, stdinBytes.length - stdinOffset);
    mem().set(stdinBytes.subarray(stdinOffset, stdinOffset + n), ptr);
    stdinOffset += n;
    return n;
  },
  fs_read_file(pptr, plen) {
    const node = lookup(readText(pptr, plen));
    if (node === null) return ERRNO.ENOENT;
    if (node.kind === "dir") return ERRNO.EISDIR; // the read stage fails, not open
    scratch(node.data);
    return ERRNO.SUCCESS;
  },
  fs_write_file(pptr, plen, dptr, dlen, append) {
    const spot = parentOf(readText(pptr, plen));
    if (spot === null || spot.dir.kind !== "dir") return ERRNO.ENOENT;
    const existing = spot.dir.entries.get(spot.name);
    if (existing !== undefined && existing.kind === "dir") return ERRNO.EISDIR;
    const data = Uint8Array.from(readBytes(dptr, dlen));
    if (existing !== undefined && append) {
      const merged = new Uint8Array(existing.data.length + data.length);
      merged.set(existing.data);
      merged.set(data, existing.data.length);
      existing.data = merged;
    } else {
      spot.dir.entries.set(spot.name, { kind: "file", data });
    }
    return ERRNO.SUCCESS;
  },
  fs_open_read(pptr, plen) {
    const node = lookup(readText(pptr, plen));
    if (node === null) return -ERRNO.ENOENT;
    handles[handleSeq] = { node, offset: 0 };
    return handleSeq++;
  },
  fs_read_line(handle) {
    const open = handles[handle];
    if (open === undefined) return -ERRNO.BADF;
    if (open.node.kind === "dir") return -ERRNO.EISDIR;
    const data = open.node.data;
    if (open.offset >= data.length) return 0; // clean EOF
    const nl = data.indexOf(0x0a, open.offset);
    const end = nl >= 0 ? nl + 1 : data.length; // raw bytes incl. "\n"; partial final line kept
    scratch(data.subarray(open.offset, end));
    open.offset = end;
    return 1;
  },
  fs_close(handle) {
    delete handles[handle]; // idempotent: closing a closed handle is a no-op
    return ERRNO.SUCCESS;
  },
  fs_stat(pptr, plen) {
    const node = lookup(readText(pptr, plen));
    if (node === null) return ERRNO.ENOENT;
    const payload = new Uint8Array(12);
    new DataView(payload.buffer).setUint32(0, node.kind === "dir" ? 1 : 0, true);
    new DataView(payload.buffer).setBigInt64(4, BigInt(node.kind === "file" ? node.data.length : 0), true);
    scratch(payload);
    return ERRNO.SUCCESS;
  },
  fs_list(pptr, plen) {
    const node = lookup(readText(pptr, plen));
    if (node === null) return ERRNO.ENOENT;
    if (node.kind !== "dir") return ERRNO.ENOTDIR;
    const names = [...node.entries.keys()].sort((a, b) => Buffer.compare(Buffer.from(a, "utf8"), Buffer.from(b, "utf8")));
    let payload = 4;
    const size = names.reduce((sum, name) => sum + 4 + Buffer.byteLength(name, "utf8"), 4);
    const out = new Uint8Array(size);
    new DataView(out.buffer).setUint32(0, names.length, true);
    for (const name of names) {
      const bytes = Buffer.from(name, "utf8");
      new DataView(out.buffer).setUint32(payload, bytes.length, true);
      out.set(bytes, payload + 4);
      payload += 4 + bytes.length;
    }
    scratch(out);
    return ERRNO.SUCCESS;
  },
  fs_make_folder(pptr, plen, parents) {
    const path = readText(pptr, plen);
    if (parents) {
      let node = fsRoot; // mkdir -p: create ancestors, tolerate existing
      for (const part of segments(path)) {
        if (node.kind !== "dir") return ERRNO.ENOTDIR;
        let child = node.entries.get(part);
        if (child === undefined) {
          child = { kind: "dir", entries: new Map() };
          node.entries.set(part, child);
        } else if (child.kind !== "dir") {
          return ERRNO.ENOTDIR;
        }
        node = child;
      }
      return ERRNO.SUCCESS;
    }
    const spot = parentOf(path);
    if (spot === null || spot.dir.kind !== "dir") return ERRNO.ENOENT;
    if (spot.dir.entries.has(spot.name)) return ERRNO.EEXIST;
    spot.dir.entries.set(spot.name, { kind: "dir", entries: new Map() });
    return ERRNO.SUCCESS;
  },
  fs_delete(pptr, plen) {
    const path = readText(pptr, plen);
    const node = lookup(path);
    if (node === null) return ERRNO.ENOENT;
    if (node.kind === "dir" && node.entries.size > 0) return ERRNO.ENOTEMPTY;
    const spot = parentOf(path);
    spot.dir.entries.delete(spot.name);
    return ERRNO.SUCCESS;
  },
  fs_delete_folder(pptr, plen, recursive) {
    const path = readText(pptr, plen);
    const node = lookup(path);
    if (node === null) return ERRNO.ENOENT; // os.RemoveAll tolerance only applies once it exists
    if (!recursive && node.kind === "dir" && node.entries.size > 0) return ERRNO.ENOTEMPTY;
    const spot = parentOf(path);
    spot.dir.entries.delete(spot.name);
    return ERRNO.SUCCESS;
  },
  fs_move(sptr, slen, dptr, dlen, overwrite) {
    const source = readText(sptr, slen);
    const target = readText(dptr, dlen);
    const srcSpot = parentOf(source);
    const src = srcSpot === null ? null : (srcSpot.dir.entries.get(srcSpot.name) ?? null);
    if (src === null) return ERRNO.ENOENT;
    const spot = parentOf(target);
    if (spot === null || spot.dir.kind !== "dir") return ERRNO.ENOENT;
    const existing = spot.dir.entries.get(spot.name);
    if (existing !== undefined && existing.kind === "dir") return ERRNO.EISDIR;
    if (existing !== undefined && !overwrite) return ERRNO.EEXIST; // module pre-checks reach this first
    spot.dir.entries.set(spot.name, src);
    srcSpot.dir.entries.delete(srcSpot.name);
    return ERRNO.SUCCESS;
  },
  env_get(kptr, klen) {
    const key = readText(kptr, klen);
    if (!envTable.has(key)) return 0;
    scratchBytes(envTable.get(key));
    return 1;
  },
  env_environ() {
    const pairs = [...envTable.entries()];
    const size = pairs.reduce(
      (sum, [k, v]) => sum + 4 + Buffer.byteLength(k, "utf8") + 4 + Buffer.byteLength(v, "utf8"),
      4,
    );
    const out = new Uint8Array(size);
    const dv = new DataView(out.buffer);
    let at = 4;
    dv.setUint32(0, pairs.length, true);
    const put = (text) => {
      const bytes = Buffer.from(text, "utf8");
      dv.setUint32(at, bytes.length, true);
      out.set(bytes, at + 4);
      at += 4 + bytes.length;
    };
    for (const [k, v] of pairs) {
      put(k);
      put(v);
    }
    scratch(out);
    return ERRNO.SUCCESS;
  },
};

const { instance } = await WebAssembly.instantiate(bytes, { env: host });
memory = instance.exports.memory;

const flush = () => process.stdout.write(Buffer.concat(chunks));

try {
  instance.exports.zhex_main();
  flush();
} catch (e) {
  flush();
  const code = instance.exports.zhex_last_error();
  if (code !== 0) {
    let message = failMessages[code] ?? "unknown fail code " + code;
    if (stagedFailCodes.has(code)) {
      const len = instance.exports.zhex_msg_len();
      message = decoder.decode(new Uint8Array(memory.buffer, 0, len));
    }
    process.stderr.write(`runtime error: ${message}\n`);
  } else if (/divide by zero|remainder by zero/.test(e.message)) {
    process.stderr.write("runtime error: division or remainder by zero\n");
  } else {
    // shortcut: any other native trap (failed cast, bounds, ...) prints verbatim; add matching above only if its text must be pinned
    process.stderr.write(`runtime error: ${e.message}\n`);
  }
  process.exit(1);
}
