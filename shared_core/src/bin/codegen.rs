use std::path::PathBuf;
use anyhow::Result;
use clap::{Parser, ValueEnum};
use crux_core::type_generation::facet::{Config, TypeRegistry};
use log::info;
use shared_core::app::LogNApp;

#[derive(Copy, Clone, PartialEq, Eq, PartialOrd, Ord, ValueEnum)]
enum Language {
    Swift,
    Kotlin,
}

impl Language {
    const fn package_name(self) -> &'static str {
        match self {
            Self::Swift => "App",
            // Irmão de sh.logn.ffi (boltffi) e de sh.logn.app, nunca filho do app: um
            // pacote dentro de outro esconde classes com o mesmo nome. O --core-package de
            // tools_i18n tem de ser este mesmo valor.
            Self::Kotlin => "sh.logn.core",
        }
    }
}

#[derive(Parser)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(short, long, value_enum)]
    language: Language,
    #[arg(short, long)]
    output_dir: PathBuf,
}

fn main() -> Result<()> {
    pretty_env_logger::init();
    let args = Args::parse();

    let typegen_app = TypeRegistry::new()
        .register_app::<LogNApp>()?
        .build()?;

    let config = Config::builder(args.language.package_name(), &args.output_dir).build();

    match args.language {
        Language::Swift => {
            info!("Typegen for Swift");
            typegen_app.swift(&config)?;
        }
        Language::Kotlin => {
            info!("Typegen for Kotlin");
            typegen_app.kotlin(&config)?;
            // O Installer escreve um build.gradle.kts de projeto JVM avulso. Quem compila
            // estes fontes é o módulo :core-shell; o arquivo não pode virar módulo.
            let manifest = args.output_dir.join("build.gradle.kts");
            if manifest.exists() {
                std::fs::remove_file(&manifest)?;
            }
        }
    }

    Ok(())
}
