// fastledWasmHost.ts: minimal host for PXLBLZ FastLED sketch modules (wasm32-wasi + Asyncify).
// Plain TypeScript (erasable syntax only), no dependencies. Runs in the browser and in Node >= 22.
//
// Provenance: PXLBLZ fastled-integration (runtime/fastledWasmHost.ts). Must match
// PXL_ABI_VERSION in runtime/pxl_runtime.cpp; a mismatch is refused in instantiate().
//
// Frame model (details: README.md):
//   init(seed)   runs setup() (delay() in setup just advances the virtual clock).
//   frame(dtMs)  advances the frame clock by dtMs and resumes loop() until it waits beyond the
//                frame time (delay), finishes a zero-time loop() iteration, busy-polls millis(),
//                or hits the runaway budget. frame(0) after the first frame is a no-op.
//   getLeds()    L1: logical CRGB of all addLeds() controllers (registration order) at the last show().
//   getWire()    L2: the bytes FastLED encoded for the wire at the last show() (brightness, color
//                correction/temperature, temporal dithering, power limit applied), RGB order.
//   getWireRaw() L2 in the controller's COLOR_ORDER (exactly what goes out on the data pin).

export const FASTLED_HOST_ABI_VERSION = 1;

export const FRAME_STATUS = { OK: 0, BUDGET: 1, NOT_INIT: 2, SKIPPED: 3 } as const;

export interface FastLedUiElement {
  id: number;
  name: string;
  /** FastLED JSON type: 'slider' | 'checkbox' | 'number' | 'button' | 'dropdown' | 'title' | 'description' | 'help' | 'audio' ... */
  type: string;
  kind: 'slider' | 'checkbox' | 'number' | 'button' | 'dropdown' | 'text' | 'other';
  group?: string;
  min?: number;
  max?: number;
  step?: number;
  options?: string[];
  value: unknown;
  defaultValue: unknown;
  raw: Record<string, unknown>;
}

export interface FastLedStrip {
  ledOffset: number;
  ledCount: number;
  wireOffset: number;
  wireLength: number;
  pin: number;
  /** EOrder value, e.g. 0o102 = GRB */
  colorOrder: number;
  colorOrderName: string;
  rgbw: number;
  isSpi: boolean;
  enabled: boolean;
}

/** Screen map set by the sketch via CLEDController::setScreenMap(XYMap | ScreenMap | w,h). */
export interface FastLedScreenMap {
  /** index of the controller in addLeds() order (-1 if it no longer exists) */
  strip: number;
  /** first LED of that controller in getLeds() */
  ledOffset: number;
  length: number;
  /** LED diameter in screen-map units (-1 = unset) */
  diameter: number;
  /** present when the map was built from an XYMap */
  xyWidth?: number;
  xyHeight?: number;
  /** XYMap type: 0 serpentine, 1 line-by-line, 2 function, 3 lookup table */
  xyType?: number;
  x: number[];
  y: number[];
}

export interface FastLedHostOptions {
  /** stdout/stderr text of the sketch (Serial.print, FastLED warnings) */
  onConsole?: (text: string, stream: 'stdout' | 'stderr') => void;
}

export interface FrameResult {
  status: number;
  /** total FastLED.show() calls so far */
  showCount: number;
  /** show() calls during this frame() */
  framesShows: number;
  nowMs: number;
}

interface Exports {
  memory: WebAssembly.Memory;
  _initialize?: () => void;
  asyncify_start_unwind: (p: number) => void;
  asyncify_stop_unwind: () => void;
  asyncify_start_rewind: (p: number) => void;
  asyncify_stop_rewind: () => void;
  asyncify_get_state?: () => number;
  pxl_abi_version: () => number;
  pxl_init: (seed: number) => number;
  pxl_frame_begin: (dtUs: number) => number;
  pxl_fiber_entry: () => void;
  pxl_frame_end: () => number;
  pxl_set_budget: (loops: number, shows: number) => void;
  pxl_led_count: () => number;
  pxl_leds_ptr: () => number;
  pxl_wire_rgb_ptr: () => number;
  pxl_wire_raw_ptr: () => number;
  pxl_wire_raw_len: () => number;
  pxl_strip_count: () => number;
  pxl_strips_ptr: () => number;
  pxl_now_ms: () => number;
  pxl_now_us_lo: () => number;
  pxl_now_us_hi: () => number;
  pxl_show_count: () => number;
  pxl_frame_show_count: () => number;
  pxl_loop_count: () => number;
  pxl_suspend_reason: () => number;
  pxl_brightness: () => number;
  pxl_ui_json: () => number;
  pxl_ui_version: () => number;
  pxl_ui_set: (p: number) => number;
  pxl_alloc: (n: number) => number;
  pxl_free: (p: number) => void;
  pxl_asyncify_data: () => number;
  pxl_set_lazy: (on: number) => void;
  /** additive exports (modules built before they existed lack them) */
  pxl_screenmap_json?: () => number;
  pxl_screenmap_version?: () => number;
}

const ORDER_NAMES: Record<number, string> = { 0o012: 'RGB', 0o021: 'RBG', 0o102: 'GRB', 0o120: 'GBR', 0o201: 'BRG', 0o210: 'BGR' };
const UTF8_DEC = new TextDecoder();
const UTF8_ENC = new TextEncoder();

/** Compile once, instantiate many times (each instance = one fresh sketch run). */
export async function compileFastLedModule(bytes: BufferSource): Promise<WebAssembly.Module> {
  return WebAssembly.compile(bytes);
}

export async function instantiateFastLedHost(
  moduleOrBytes: WebAssembly.Module | BufferSource,
  options: FastLedHostOptions = {},
): Promise<FastLedHost> {
  const module = moduleOrBytes instanceof WebAssembly.Module ? moduleOrBytes : await WebAssembly.compile(moduleOrBytes);
  const host = new FastLedHost(options);
  const instance = await WebAssembly.instantiate(module, host.imports(module));
  host.attach(instance);
  return host;
}

const enum_ASYNC_NORMAL = 0;
const enum_ASYNC_UNWINDING = 1;
const enum_ASYNC_REWINDING = 2;

export class FastLedHost {
  private ex!: Exports;
  private opts: FastLedHostOptions;
  private asyncData = 0;
  private asyncState = enum_ASYNC_NORMAL;
  private started = false;
  private dead: Error | null = null;
  private uiDefaults = new Map<number, unknown>();
  private uiCacheVersion = -1;
  private uiCache: FastLedUiElement[] = [];
  private textBuf: Record<number, string> = { 1: '', 2: '' };
  private initialized = false;
  private accMs = 0;
  private issuedUs = 0;

  constructor(opts: FastLedHostOptions) {
    this.opts = opts;
  }

  // ------------------------------------------------------------------ instantiation
  imports(module: WebAssembly.Module): WebAssembly.Imports {
    const self = this;
    const ENOSYS = 52, EBADF = 8;
    const mem = () => new DataView(self.ex.memory.buffer);
    const u8 = () => new Uint8Array(self.ex.memory.buffer);
    let rng = 0x9e3779b9;
    const wasi: Record<string, (...a: any[]) => any> = {
      fd_write(fd: number, iovs: number, iovsLen: number, nwritten: number) {
        const dv = mem();
        let total = 0;
        let text = '';
        for (let i = 0; i < iovsLen; i++) {
          const p = dv.getUint32(iovs + i * 8, true), n = dv.getUint32(iovs + i * 8 + 4, true);
          text += UTF8_DEC.decode(u8().subarray(p, p + n));
          total += n;
        }
        dv.setUint32(nwritten, total, true);
        if (fd === 1 || fd === 2) self.emitText(fd, text);
        return 0;
      },
      fd_close: () => 0,
      fd_seek: () => ENOSYS,
      fd_read: () => ENOSYS,
      fd_fdstat_get: (fd: number, p: number) => {
        if (fd > 2) return EBADF;
        const dv = mem();
        dv.setUint8(p, 2); // character device
        dv.setUint16(p + 2, 0, true);
        dv.setBigUint64(p + 8, 0n, true);
        dv.setBigUint64(p + 16, 0n, true);
        return 0;
      },
      fd_prestat_get: () => EBADF,
      fd_prestat_dir_name: () => EBADF,
      environ_sizes_get: (c: number, s: number) => { mem().setUint32(c, 0, true); mem().setUint32(s, 0, true); return 0; },
      environ_get: () => 0,
      args_sizes_get: (c: number, s: number) => { mem().setUint32(c, 0, true); mem().setUint32(s, 0, true); return 0; },
      args_get: () => 0,
      // Deterministic: the virtual clock, never the wall clock.
      clock_time_get: (_id: number, _prec: bigint, out: number) => {
        const us = BigInt(self.ex.pxl_now_us_hi() >>> 0) * 4294967296n + BigInt(self.ex.pxl_now_us_lo() >>> 0);
        mem().setBigUint64(out, us * 1000n, true);
        return 0;
      },
      clock_res_get: (_id: number, out: number) => { mem().setBigUint64(out, 1000n, true); return 0; },
      random_get: (p: number, n: number) => {
        const b = u8();
        for (let i = 0; i < n; i++) { rng ^= rng << 13; rng ^= rng >>> 17; rng ^= rng << 5; b[p + i] = rng & 0xff; }
        return 0;
      },
      proc_exit: (code: number) => { throw new Error(`sketch called exit(${code})`); },
      sched_yield: () => 0,
      poll_oneoff: () => ENOSYS,
    };
    const env: Record<string, (...a: any[]) => any> = {
      pxl_suspend: () => self.onSuspend(),
    };
    // Unknown imports become ENOSYS stubs (no filesystem / network in the sandbox).
    const out: Record<string, Record<string, any>> = {};
    for (const imp of WebAssembly.Module.imports(module)) {
      if (imp.kind !== 'function') continue;
      const table = imp.module === 'env' ? env : wasi;
      const fn = table[imp.name] ?? (() => ENOSYS);
      (out[imp.module] ??= {})[imp.name] = fn;
    }
    return out;
  }

  attach(instance: WebAssembly.Instance): void {
    this.ex = instance.exports as unknown as Exports;
    const abi = this.ex.pxl_abi_version?.();
    if (abi !== FASTLED_HOST_ABI_VERSION) {
      throw new Error(`FastLED module ABI ${abi} does not match host ABI ${FASTLED_HOST_ABI_VERSION}; recompile the sketch or update fastledWasmHost.ts`);
    }
    if (!this.ex.asyncify_start_unwind) throw new Error('FastLED module was not processed with binaryen --asyncify');
    this.ex._initialize?.();
    this.asyncData = this.ex.pxl_asyncify_data();
  }

  private emitText(fd: number, text: string) {
    const cb = this.opts.onConsole;
    if (!cb) return;
    const buf = this.textBuf[fd] + text;
    const nl = buf.lastIndexOf('\n');
    if (nl < 0) { this.textBuf[fd] = buf; return; }
    this.textBuf[fd] = buf.slice(nl + 1);
    cb(buf.slice(0, nl), fd === 1 ? 'stdout' : 'stderr');
  }

  // ------------------------------------------------------------------ asyncify fiber
  private onSuspend(): void {
    if (this.asyncState === enum_ASYNC_REWINDING) {
      this.ex.asyncify_stop_rewind();
      this.asyncState = enum_ASYNC_NORMAL;
      return;
    }
    // Reset the save-stack cursor and start unwinding to pxl_fiber_entry's caller (frame()).
    this.asyncData = this.ex.pxl_asyncify_data();
    this.ex.asyncify_start_unwind(this.asyncData);
    this.asyncState = enum_ASYNC_UNWINDING;
  }

  // ------------------------------------------------------------------ public API
  abiVersion(): number { return this.ex.pxl_abi_version(); }

  /** Runs setup(). seed seeds Arduino random()/rand() (avr-libc algorithm, default 1). FastLED's random8/16 keep their own default seed. */
  init(seed = 1): void {
    if (this.initialized) throw new Error('init() can only be called once per instance (instantiate a new host to restart)');
    this.guard(() => this.ex.pxl_init(seed >>> 0));
    this.initialized = true;
    for (const el of this.readUi()) if (!this.uiDefaults.has(el.id)) this.uiDefaults.set(el.id, el.value);
  }

  /**
   * Advances the virtual frame clock by dtMs and runs the sketch until the next frame boundary.
   * Returns the frame status (FRAME_STATUS.BUDGET = runaway loop detected; the sketch stays resumable).
   * Throws if the sketch trapped (the instance is dead afterwards).
   */
  frame(dtMs: number): FrameResult {
    if (this.dead) throw this.dead;
    if (!this.initialized) throw new Error('init() first');
    // Frame clock in whole microseconds without drift: round the accumulated ms sum, not each dt.
    this.accMs += Math.max(0, dtMs);
    let whole = Math.round(this.accMs * 1000) - this.issuedUs;
    if (whole > 0xffffffff) whole = 0xffffffff;
    this.issuedUs += whole;
    return this.frameUs(whole);
  }

  /** Same as frame() with an exact integer dt in microseconds (does not touch the ms accumulator). */
  frameUs(dtUs: number): FrameResult {
    if (this.dead) throw this.dead;
    if (!this.initialized) throw new Error('init() first');
    const whole = Math.max(0, Math.min(0xffffffff, Math.floor(dtUs)));
    const ex = this.ex;
    const status = this.guard(() => {
      if (ex.pxl_frame_begin(whole >>> 0)) {
        if (!this.started) {
          this.started = true;
          ex.pxl_fiber_entry();
        } else {
          ex.asyncify_start_rewind(this.asyncData);
          this.asyncState = enum_ASYNC_REWINDING;
          ex.pxl_fiber_entry();
        }
        if (this.asyncState !== enum_ASYNC_UNWINDING) throw new Error('sketch fiber returned unexpectedly');
        ex.asyncify_stop_unwind();
        this.asyncState = enum_ASYNC_NORMAL;
      }
      return ex.pxl_frame_end();
    });
    return { status, showCount: ex.pxl_show_count(), framesShows: ex.pxl_frame_show_count(), nowMs: ex.pxl_now_ms() };
  }

  private guard<T>(fn: () => T): T {
    try {
      return fn();
    } catch (e) {
      this.dead = e instanceof Error ? e : new Error(String(e));
      this.dead.message = `FastLED sketch crashed: ${this.dead.message}`;
      throw this.dead;
    }
  }

  ledCount(): number { return this.ex.pxl_led_count(); }

  /** L1 (RGB888, 3*ledCount). View into wasm memory: copy it if you keep it across frame() calls. */
  getLeds(): Uint8Array {
    const p = this.ex.pxl_leds_ptr();
    return new Uint8Array(this.ex.memory.buffer, p, this.ledCount() * 3);
  }

  /** L2 in RGB order (3*ledCount). View into wasm memory. */
  getWire(): Uint8Array {
    const p = this.ex.pxl_wire_rgb_ptr();
    return new Uint8Array(this.ex.memory.buffer, p, this.ledCount() * 3);
  }

  /** L2 exactly as on the data pins (controller COLOR_ORDER, RGBW expansion), all strips concatenated. */
  getWireRaw(): Uint8Array {
    const p = this.ex.pxl_wire_raw_ptr();
    return new Uint8Array(this.ex.memory.buffer, p, this.ex.pxl_wire_raw_len());
  }

  getStrips(): FastLedStrip[] {
    const n = this.ex.pxl_strip_count();
    const dv = new DataView(this.ex.memory.buffer, this.ex.pxl_strips_ptr(), n * 40);
    const out: FastLedStrip[] = [];
    for (let i = 0; i < n; i++) {
      const g = (k: number) => dv.getInt32(i * 40 + k * 4, true);
      out.push({
        ledOffset: g(0), ledCount: g(1), wireOffset: g(2), wireLength: g(3), pin: g(4), colorOrder: g(5),
        colorOrderName: ORDER_NAMES[g(5)] ?? `0o${g(5).toString(8)}`, rgbw: g(6), isSpi: !!g(7), enabled: !!g(8),
      });
    }
    return out;
  }

  /** Screen maps registered by the sketch (empty if none, or if the module predates this export). */
  getScreenMaps(): FastLedScreenMap[] {
    if (!this.ex.pxl_screenmap_json) return [];
    const v = this.ex.pxl_screenmap_version?.() ?? 0;
    if (v === this.smVersion) return this.smCache;
    const p = this.ex.pxl_screenmap_json();
    const b = new Uint8Array(this.ex.memory.buffer);
    let e = p;
    while (b[e]) e++;
    try { this.smCache = JSON.parse(UTF8_DEC.decode(b.subarray(p, e))); } catch { this.smCache = []; }
    this.smVersion = v;
    return this.smCache;
  }
  private smVersion = -1;
  private smCache: FastLedScreenMap[] = [];

  millis(): number { return this.ex.pxl_now_ms(); }
  showCount(): number { return this.ex.pxl_show_count(); }
  loopCount(): number { return this.ex.pxl_loop_count(); }
  brightness(): number { return this.ex.pxl_brightness(); }

  /** Lazy wire encoding (default on): encode L2 once per getWire() instead of on every show(). Same bytes. Call before init(). */
  setLazyEncoding(on: boolean): void { this.ex.pxl_set_lazy(on ? 1 : 0); }

  /** Runaway protection: max loop() iterations / show() calls per frame() (defaults 100000 / 20000). */
  setBudget(loops: number, shows: number): void { this.ex.pxl_set_budget(loops >>> 0, shows >>> 0); }

  // ------------------------------------------------------------------ UI elements
  private readUi(): FastLedUiElement[] {
    const p = this.ex.pxl_ui_json();
    const b = new Uint8Array(this.ex.memory.buffer);
    let e = p;
    while (b[e]) e++;
    let arr: any[] = [];
    try { arr = JSON.parse(UTF8_DEC.decode(b.subarray(p, e))); } catch { arr = []; }
    return (Array.isArray(arr) ? arr : []).map((raw) => {
      const type = String(raw.type ?? '');
      const kind: FastLedUiElement['kind'] = type === 'slider' ? 'slider' : type === 'checkbox' ? 'checkbox'
        : type === 'number' || type === 'number_field' ? 'number' : type === 'button' ? 'button'
        : type === 'dropdown' ? 'dropdown' : ['title', 'description', 'help'].includes(type) ? 'text' : 'other';
      const id = Number(raw.id);
      return {
        id, name: String(raw.name ?? ''), type, kind, group: raw.group || undefined,
        min: typeof raw.min === 'number' ? raw.min : undefined,
        max: typeof raw.max === 'number' ? raw.max : undefined,
        step: typeof raw.step === 'number' ? raw.step : undefined,
        options: Array.isArray(raw.options) ? raw.options.map(String) : undefined,
        value: raw.value,
        defaultValue: this.uiDefaults.has(id) ? this.uiDefaults.get(id) : raw.value,
        raw,
      };
    });
  }

  /** FastLED UI elements (UISlider, UICheckbox, UINumberField, UIButton, UIDropdown, UITitle, ...). */
  getUi(): FastLedUiElement[] {
    const v = this.ex.pxl_ui_version();
    if (v !== this.uiCacheVersion) {
      this.uiCache = this.readUi();
      for (const el of this.uiCache) if (!this.uiDefaults.has(el.id)) { this.uiDefaults.set(el.id, el.value); el.defaultValue = el.value; }
      this.uiCacheVersion = this.ex.pxl_ui_version();
      // FastLED re-serializes every element whenever one changes on the sketch side, so a newer
      // JSON already contains the values set from JS; only overlay values set after it.
      if (this.uiCacheVersion !== this.uiOverrideVersion) this.uiOverrides.clear();
    }
    if (this.uiOverrides.size) {
      for (const el of this.uiCache) if (this.uiOverrides.has(el.id)) el.value = this.uiOverrides.get(el.id);
    }
    return this.uiCache;
  }

  private uiOverrides = new Map<number, unknown>();
  private uiOverrideVersion = -1;

  /** Set one element (by numeric id or by name). Takes effect immediately (before the next loop() code runs). */
  setUi(idOrName: number | string, value: number | boolean | string): boolean {
    return this.setUiMany({ [String(idOrName)]: value });
  }

  setUiMany(values: Record<string, number | boolean | string>): boolean {
    const bytes = UTF8_ENC.encode(JSON.stringify(values));
    const p = this.ex.pxl_alloc(bytes.length + 1);
    const b = new Uint8Array(this.ex.memory.buffer, p, bytes.length + 1);
    b.set(bytes);
    b[bytes.length] = 0;
    const r = this.guard(() => this.ex.pxl_ui_set(p));
    this.ex.pxl_free(p);
    // FastLED does not echo UI-originated changes as JSON; mirror them (clamped like FastLED does).
    const els = this.getUi();
    for (const [key, val] of Object.entries(values)) {
      const el = els.find((e) => String(e.id) === key) ?? els.find((e) => e.name === key);
      if (!el) continue;
      let v: unknown = val;
      if (typeof val === 'number' && (el.kind === 'slider' || el.kind === 'number')) {
        v = Math.min(el.max ?? Infinity, Math.max(el.min ?? -Infinity, val));
      }
      this.uiOverrides.set(el.id, v);
      el.value = v;
    }
    this.uiOverrideVersion = this.ex.pxl_ui_version();
    return r === 0;
  }
}
