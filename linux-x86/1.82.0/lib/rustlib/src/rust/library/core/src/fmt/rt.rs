#![allow(missing_debug_implementations)]
#![unstable(feature = "fmt_internals", reason = "internal to format_args!", issue = "none")]

//! These are the lang items used by format_args!().

use super::*;
use crate::hint::unreachable_unchecked;
use crate::ptr::NonNull;

#[lang = "format_placeholder"]
#[derive(Copy, Clone)]
pub struct Placeholder {
    pub position: usize,
    pub fill: char,
    pub align: Alignment,
    pub flags: u32,
    pub precision: Count,
    pub width: Count,
}

impl Placeholder {
    #[inline(always)]
    pub const fn new(
        position: usize,
        fill: char,
        align: Alignment,
        flags: u32,
        precision: Count,
        width: Count,
    ) -> Self {
        Self { position, fill, align, flags, precision, width }
    }
}

#[lang = "format_alignment"]
#[derive(Copy, Clone, PartialEq, Eq)]
pub enum Alignment {
    Left,
    Right,
    Center,
    Unknown,
}

/// Used by [width](https://doc.rust-lang.org/std/fmt/#width)
/// and [precision](https://doc.rust-lang.org/std/fmt/#precision) specifiers.
#[lang = "format_count"]
#[derive(Copy, Clone)]
pub enum Count {
    /// Specified with a literal number, stores the value
    Is(usize),
    /// Specified using `$` and `*` syntaxes, stores the index into `args`
    Param(usize),
    /// Not specified
    Implied,
}

// This needs to match the order of flags in compiler/rustc_ast_lowering/src/format.rs.
#[derive(Copy, Clone)]
pub(super) enum Flag {
    SignPlus,
    SignMinus,
    Alternate,
    SignAwareZeroPad,
    DebugLowerHex,
    DebugUpperHex,
}

#[derive(Copy, Clone)]
enum ArgumentType<'a> {
    Placeholder {
        // INVARIANT: `formatter` has type `fn(&T, _) -> _` for some `T`, and `value`
        // was derived from a `&'a T`.
        value: NonNull<()>,
        formatter: unsafe fn(NonNull<()>, &mut Formatter<'_>) -> Result,
        _lifetime: PhantomData<&'a ()>,
    },
    Count(usize),
}

/// This struct represents a generic "argument" which is taken by format_args!().
///
/// This can be either a placeholder argument or a count argument.
/// * A placeholder argument contains a function to format the given value. At
///   compile time it is ensured that the function and the value have the correct
///   types, and then this struct is used to canonicalize arguments to one type.
///   Placeholder arguments are essentially an optimized partially applied formatting
///   function, equivalent to `exists T.(&T, fn(&T, &mut Formatter<'_>) -> Result`.
/// * A count argument contains a count for dynamic formatting parameters like
///   precision and width.
#[lang = "format_argument"]
#[derive(Copy, Clone)]
pub struct Argument<'a> {
    ty: ArgumentType<'a>,
}

macro_rules! argument_new {
    ($t:ty, $x:expr, $f:expr) => {
        Argument {
            // INVARIANT: this creates an `ArgumentType<'b>` from a `&'b T` and
            // a `fn(&T, ...)`, so the invariant is maintained.
            ty: ArgumentType::Placeholder {
                value: NonNull::<$t>::from($x).cast(),
                // The Rust ABI considers all pointers to be equivalent, so transmuting a fn(&T) to
                // fn(NonNull<()>) and calling it with a NonNull<()> that points at a T is allowed.
                // However, the CFI sanitizer does not allow this, and triggers a crash when it
                // happens.
                //
                // To avoid this crash, we use a helper function when CFI is enabled. To avoid the
                // cost of this helper function (mainly code-size) when it is not needed, we
                // transmute the function pointer otherwise.
                //
                // This is similar to what the Rust compiler does internally with vtables when KCFI
                // is enabled, where it generates trampoline functions that only serve to adjust the
                // expected type of the argument. `ArgumentType::Placeholder` is a bit like a
                // manually constructed trait object, so it is not surprising that the same approach
                // has to be applied here as well.
                //
                // It is still considered problematic (from the Rust side) that CFI rejects entirely
                // legal Rust programs, so we do not consider anything done here a stable guarantee,
                // but meanwhile we carry this work-around to keep Rust compatible with CFI and
                // KCFI.
                #[cfg(not(any(sanitize = "cfi", sanitize = "kcfi")))]
                formatter: {
                    let f: fn(&$t, &mut Formatter<'_>) -> Result = $f;
                    // SAFETY: This is only called with `value`, which has the right type.
                    unsafe { core::mem::transmute(f) }
                },
                #[cfg(any(sanitize = "cfi", sanitize = "kcfi"))]
                formatter: |ptr: NonNull<()>, fmt: &mut Formatter<'_>| {
                    let func = $f;
                    // SAFETY: This is the same type as the `value` field.
                    let r = unsafe { ptr.cast::<$t>().as_ref() };
                    (func)(r, fmt)
                },
                _lifetime: PhantomData,
            },
        }
    };
}

#[rustc_diagnostic_item = "ArgumentMethods"]
impl<'a> Argument<'a> {
    #[inline(always)]
    pub fn new_display<'b, T: Display>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as Display>::fmt)
    }
    #[inline(always)]
    pub fn new_debug<'b, T: Debug>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as Debug>::fmt)
    }
    #[inline(always)]
    pub fn new_debug_noop<'b, T: Debug>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, |_, _| Ok(()))
    }
    #[inline(always)]
    pub fn new_octal<'b, T: Octal>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as Octal>::fmt)
    }
    #[inline(always)]
    pub fn new_lower_hex<'b, T: LowerHex>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as LowerHex>::fmt)
    }
    #[inline(always)]
    pub fn new_upper_hex<'b, T: UpperHex>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as UpperHex>::fmt)
    }
    #[inline(always)]
    pub fn new_pointer<'b, T: Pointer>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as Pointer>::fmt)
    }
    #[inline(always)]
    pub fn new_binary<'b, T: Binary>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as Binary>::fmt)
    }
    #[inline(always)]
    pub fn new_lower_exp<'b, T: LowerExp>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as LowerExp>::fmt)
    }
    #[inline(always)]
    pub fn new_upper_exp<'b, T: UpperExp>(x: &'b T) -> Argument<'_> {
        argument_new!(T, x, <T as UpperExp>::fmt)
    }
    #[inline(always)]
    pub fn from_usize(x: &usize) -> Argument<'_> {
        Argument { ty: ArgumentType::Count(*x) }
    }

    /// Format this placeholder argument.
    ///
    /// # Safety
    ///
    /// This argument must actually be a placeholder argument.
    #[inline(always)]
    pub(super) unsafe fn fmt(&self, f: &mut Formatter<'_>) -> Result {
        match self.ty {
            // SAFETY:
            // Because of the invariant that if `formatter` had the type
            // `fn(&T, _) -> _` then `value` has type `&'b T` where `'b` is
            // the lifetime of the `ArgumentType`, and because references
            // and `NonNull` are ABI-compatible, this is completely equivalent
            // to calling the original function passed to `new` with the
            // original reference, which is sound.
            ArgumentType::Placeholder { formatter, value, .. } => unsafe { formatter(value, f) },
            // SAFETY: the caller promised this.
            ArgumentType::Count(_) => unsafe { unreachable_unchecked() },
        }
    }

    #[inline(always)]
    pub(super) fn as_usize(&self) -> Option<usize> {
        match self.ty {
            ArgumentType::Count(count) => Some(count),
            ArgumentType::Placeholder { .. } => None,
        }
    }

    /// Used by `format_args` when all arguments are gone after inlining,
    /// when using `&[]` would incorrectly allow for a bigger lifetime.
    ///
    /// This fails without format argument inlining, and that shouldn't be different
    /// when the argument is inlined:
    ///
    /// ```compile_fail,E0716
    /// let f = format_args!("{}", "a");
    /// println!("{f}");
    /// ```
    #[inline(always)]
    pub fn none() -> [Self; 0] {
        []
    }
}

/// This struct represents the unsafety of constructing an `Arguments`.
/// It exists, rather than an unsafe function, in order to simplify the expansion
/// of `format_args!(..)` and reduce the scope of the `unsafe` block.
#[lang = "format_unsafe_arg"]
pub struct UnsafeArg {
    _private: (),
}

impl UnsafeArg {
    /// See documentation where `UnsafeArg` is required to know when it is safe to
    /// create and use `UnsafeArg`.
    #[inline(always)]
    pub unsafe fn new() -> Self {
        Self { _private: () }
    }
}
