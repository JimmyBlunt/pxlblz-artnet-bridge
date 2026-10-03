// Native reference only: the libm functions that the wasm32-wasi build takes from zig's own
// libc (lib/c/math.zig, std.math), exported for the x86_64-windows-gnu reference so that
// both builds use bit-identical implementations (mingw-w64 has its own, different ones).
// Bodies mirror zig 0.16 lib/c/math.zig (wasi branch). See toolchain.mjs NATIVE_LIBM.
const std = @import("std");
const math = std.math;

export fn acos(x: f64) f64 { return math.acos(x); }
export fn acosf(x: f32) f32 { return math.acos(x); }
export fn acoshf(x: f32) f32 { return math.acosh(x); }
export fn asin(x: f64) f64 { return math.asin(x); }
export fn atan(x: f64) f64 { return math.atan(x); }
export fn atanf(x: f32) f32 { return math.atan(x); }
export fn cbrt(x: f64) f64 { return math.cbrt(x); }
export fn cbrtf(x: f32) f32 { return math.cbrt(x); }
export fn cosh(x: f64) f64 { return math.cosh(x); }
export fn coshf(x: f32) f32 { return math.cosh(x); }
export fn exp10(x: f64) f64 { return math.pow(f64, 10.0, x); }
export fn exp10f(x: f32) f32 { return math.pow(f32, 10.0, x); }
export fn hypot(x: f64, y: f64) f64 { return math.hypot(x, y); }
export fn tanh(x: f64) f64 { return math.tanh(x); }
export fn tanhf(x: f32) f32 { return math.tanh(x); }
