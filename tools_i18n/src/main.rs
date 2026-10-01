//! Turns `i18n/` into native string resources, one shell at a time.
//!
//! A string is written once, in one file, in one language per file. Every shell
//! reads the result of this rather than a copy of it — which is the only way
//! `shells × languages` stops being the number of places a sentence lives.

use std::path::PathBuf;

use anyhow::Result;
use clap::{Parser, ValueEnum};

mod catalog;
mod kotlin;
mod swift;

use catalog::Catalog;

#[derive(Copy, Clone, PartialEq, Eq, PartialOrd, Ord, ValueEnum)]
enum Language {
    Swift,
    Kotlin,
}

#[derive(Parser)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(short, long, value_enum)]
    language: Language,

    /// Where the generated resources go. For Swift a folder compiled into the
    /// app; for Kotlin an Android `src/main`.
    #[arg(short, long)]
    output_dir: PathBuf,

    /// The catalog to read.
    #[arg(short, long, default_value = "i18n")]
    catalog: PathBuf,

    /// Android application id, used for the `R` import and the package of the
    /// generated accessors.
    #[arg(long, default_value = "sh.logn.app")]
    android_package: String,

    /// Package the generated Core types live in, so the Kotlin mappers can
    /// import the enums they extend. Must match `Language::Kotlin.package_name()`
    /// in `shared_core/src/bin/codegen.rs`.
    #[arg(long, default_value = "sh.logn.core")]
    core_package: String,
}

fn main() -> Result<()> {
    let args = Args::parse();
    let catalog = Catalog::load(&args.catalog)?;

    let keys: usize = catalog.groups.iter().map(|group| group.keys.len()).sum();
    let languages: Vec<&str> = catalog.locales.iter().map(|l| l.tag.as_str()).collect();

    match args.language {
        Language::Swift => swift::emit(&catalog, &args.output_dir)?,
        Language::Kotlin => kotlin::emit(
            &catalog,
            &args.output_dir,
            &args.android_package,
            &args.core_package,
        )?,
    }

    println!(
        "  {keys} strings × {} languages ({}, falling back to {}) -> {}",
        languages.len(),
        languages.join(", "),
        catalog.source_language,
        args.output_dir.display()
    );

    Ok(())
}
