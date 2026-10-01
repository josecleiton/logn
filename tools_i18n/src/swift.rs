//! Apple output: a String Catalog, typed accessors, and the mappers that turn a
//! Core enum into copy.
//!
//! Nothing here is a Swift package. The generated files are compiled straight
//! into the app and the widget extension, so each bundle carries its own copy of
//! the catalog and `Bundle.main` resolves correctly in both — a package would
//! have meant `Bundle.module` and a product the widget cannot see.

use std::path::Path;

use anyhow::{Context as _, Result};
use serde_json::{Map, Value, json};

use crate::catalog::{Catalog, Group, Key, Message, PlaceholderKind, SWIFT, render};

/// `write!` into a `String` cannot fail, and saying so at every call site would
/// bury what the emitter is actually writing.
macro_rules! push {
    ($out:expr, $($arg:tt)*) => {{
        use std::fmt::Write as _;
        let _ = write!($out, $($arg)*);
    }};
}

const BANNER: &str = "// Generated from i18n/ by `just apple/i18n`. Do not edit.\n\
                      //\n\
                      // The copy lives in i18n/locales/*.toml; the keys in i18n/keys.toml.\n";

pub fn emit(catalog: &Catalog, out: &Path) -> Result<()> {
    std::fs::create_dir_all(out).with_context(|| format!("creating {}", out.display()))?;

    write(&out.join("Strings.swift"), &strings_file(catalog))?;
    write(&out.join("StringsMappers.swift"), &mappers_file(catalog))?;
    write(
        &out.join("Localizable.xcstrings"),
        &string_catalog(catalog)?,
    )?;
    info_plist_strings(catalog, out)?;

    Ok(())
}

// ---------------------------------------------------------------------------
// Strings.swift — the typed way in
// ---------------------------------------------------------------------------

fn strings_file(catalog: &Catalog) -> String {
    let mut out = String::from(BANNER);
    out.push_str("\nimport Foundation\n\n");
    out.push_str(
        "/// Every user-facing string in the app, resolved against `Localizable.xcstrings`.\n",
    );
    // The project defaults to MainActor isolation. A string lookup has no state
    // to protect and is read from background work and from AppIntents, which are
    // nonisolated themselves.
    out.push_str("nonisolated enum Str {\n");

    let mut first = true;
    for group in catalog.groups.iter().filter(|group| !group.info_plist) {
        if !first {
            out.push('\n');
        }
        first = false;

        push_doc(&mut out, 4, group.comment.as_deref());
        push!(out, "    enum {} {{\n", type_name(&group.name));

        for (index, key) in group.keys.iter().enumerate() {
            if index > 0 {
                out.push('\n');
            }
            out.push_str(&accessor(catalog, group, key));
        }

        out.push_str("    }\n");
    }

    out.push_str("}\n");

    let resource_groups: Vec<&Group> = catalog
        .groups
        .iter()
        .filter(|group| group.resource && !group.info_plist)
        .collect();

    if !resource_groups.is_empty() {
        out.push_str(
            "\n/// The same strings as `LocalizedStringResource`, for the APIs that take one:\n\
             /// ActivityKit alerts, AppIntents, anything read outside the process. A\n\
             /// resource is a reference to the catalog rather than a resolved String, so\n\
             /// the reader's language decides, not the language at the call site.\n\
             ///\n\
             /// One exception is not reachable this way: an `AppIntent`'s `title` is\n\
             /// extracted from source at build time, and `appintentsmetadataprocessor`\n\
             /// rejects anything but a literal. Those keys are spelled out in the shell.\n",
        );
        out.push_str("nonisolated enum StrRes {\n");

        for (index, group) in resource_groups.iter().enumerate() {
            if index > 0 {
                out.push('\n');
            }
            push_doc(&mut out, 4, group.comment.as_deref());
            push!(out, "    enum {} {{\n", type_name(&group.name));

            for (key_index, key) in group.keys.iter().enumerate() {
                if key_index > 0 {
                    out.push('\n');
                }

                let id = format!("{}.{}", group.name, key.name);
                push_doc(&mut out, 8, key.comment.as_deref());
                push_doc(&mut out, 8, Some(&source_preview(catalog, &id)));
                push!(
                    out,
                    "        static var {}: LocalizedStringResource {{ LocalizedStringResource(\"{id}\", bundle: .main) }}\n",
                    member_name(&key.name)
                );
            }

            out.push_str("    }\n");
        }

        out.push_str("}\n");
    }

    out.push_str(
        "\n// MARK: - Lookup\n\n\
         private nonisolated func localized(_ key: String) -> String {\n\
         \x20   NSLocalizedString(key, bundle: .main, comment: \"\")\n\
         }\n\n\
         /// `String(format:locale:)` rather than plain interpolation: it is what\n\
         /// resolves a String Catalog's plural variations, and what formats numbers\n\
         /// the way the reader's locale writes them.\n\
         private nonisolated func localized(_ key: String, _ arguments: [any CVarArg]) -> String {\n\
         \x20   String(format: localized(key), locale: .current, arguments: arguments)\n\
         }\n",
    );

    out
}

fn accessor(catalog: &Catalog, group: &Group, key: &Key) -> String {
    let id = format!("{}.{}", group.name, key.name);
    let mut out = String::new();

    push_doc(&mut out, 8, key.comment.as_deref());
    push_doc(&mut out, 8, Some(&source_preview(catalog, &id)));

    let name = member_name(&key.name);

    if key.placeholders.is_empty() {
        push!(
            out,
            "        static var {name}: String {{ localized(\"{id}\") }}\n"
        );

        return out;
    }

    let parameters = key
        .placeholders
        .iter()
        .map(|placeholder| format!("_ {}: {}", placeholder.name, swift_type(placeholder.kind)))
        .collect::<Vec<_>>()
        .join(", ");
    let arguments = key
        .placeholders
        .iter()
        .map(|placeholder| placeholder.name.clone())
        .collect::<Vec<_>>()
        .join(", ");

    push!(
        out,
        "        static func {name}({parameters}) -> String {{ localized(\"{id}\", [{arguments}]) }}\n"
    );

    out
}

// ---------------------------------------------------------------------------
// StringsMappers.swift — Core enum to copy
// ---------------------------------------------------------------------------

/// One extension per Core type, carrying every member declared for it.
///
/// These `switch`es are the whole reason the catalog can promise the shells stay
/// in step: they are written from the catalog but compiled against the Rust
/// types, so a variant added to the core and a variant left out of the catalog
/// are both build failures, in every shell, on the same commit.
fn mappers_file(catalog: &Catalog) -> String {
    let by_type = catalog.groups_by_enum();

    let mut out = String::from(BANNER);
    // One import per namespace the catalog actually binds to, rather than a
    // fixed `Match`: the day a second namespace appeared, the generated
    // mappers stopped compiling and the generator was the last place anyone
    // would look.
    let mut namespaces: Vec<&str> = by_type
        .keys()
        .filter_map(|enum_type| enum_type.split('.').next())
        .collect();
    namespaces.sort_unstable();
    namespaces.dedup();
    out.push('\n');
    for namespace in namespaces {
        push!(out, "import {namespace}\n");
    }

    for (enum_type, groups) in &by_type {
        push!(out, "\nextension {enum_type} {{\n");

        for (index, group) in groups.iter().enumerate() {
            if index > 0 {
                out.push('\n');
            }

            push_doc(&mut out, 4, group.comment.as_deref());
            push!(
                out,
                "    var {}: String {{\n        switch self {{\n",
                member_name(&group.property)
            );

            for key in &group.keys {
                // A variant whose copy interpolates what the variant carries: bind the
                // associated values and hand them to the accessor. Without this the
                // mapper names the function where a `String` belongs, which does not
                // compile — the safeguard working, but only once somebody has written
                // such a key, and `submitFailure.refused` is the first.
                let bindings = key
                    .placeholders
                    .iter()
                    .map(|placeholder| format!("let {}", placeholder.name))
                    .collect::<Vec<_>>()
                    .join(", ");
                let arguments = key
                    .placeholders
                    .iter()
                    .map(|placeholder| convert(&placeholder.name, placeholder.kind))
                    .collect::<Vec<_>>()
                    .join(", ");
                let pattern = if bindings.is_empty() {
                    String::new()
                } else {
                    format!("({bindings})")
                };
                let call = if arguments.is_empty() {
                    String::new()
                } else {
                    format!("({arguments})")
                };

                push!(
                    out,
                    "        case .{}{pattern}: Str.{}.{}{call}\n",
                    member_name(&key.name),
                    type_name(&group.name),
                    member_name(&key.name)
                );
            }

            out.push_str("        }\n    }\n");
        }

        out.push_str("}\n");
    }

    out
}

// ---------------------------------------------------------------------------
// Localizable.xcstrings
// ---------------------------------------------------------------------------

fn string_catalog(catalog: &Catalog) -> Result<String> {
    let mut strings = Map::new();

    for group in catalog.groups.iter().filter(|group| !group.info_plist) {
        for key in &group.keys {
            let id = format!("{}.{}", group.name, key.name);
            let mut entry = Map::new();

            if let Some(comment) = &key.comment {
                entry.insert("comment".to_owned(), json!(comment));
            }
            // The catalog is written by this generator, so Xcode must not try to
            // reconcile it against literals it finds in the source.
            entry.insert("extractionState".to_owned(), json!("manual"));

            let mut localizations = Map::new();
            for locale in &catalog.locales {
                let message = locale.messages.get(&id).expect("validated while loading");
                localizations.insert(locale.tag.clone(), localization(message, key));
            }
            entry.insert("localizations".to_owned(), Value::Object(localizations));

            strings.insert(id, Value::Object(entry));
        }
    }

    let document = json!({
        "sourceLanguage": catalog.source_language,
        "strings": Value::Object(strings),
        "version": "1.0",
    });

    Ok(format!("{}\n", serde_json::to_string_pretty(&document)?))
}

fn localization(message: &Message, key: &Key) -> Value {
    match message {
        Message::Simple(text) => json!({
            "stringUnit": string_unit(text, key),
        }),
        Message::Plural(forms) => {
            let mut variations: Map<String, Value> = forms
                .iter()
                .map(|(category, text)| {
                    (
                        category.clone(),
                        json!({ "stringUnit": string_unit(text, key) }),
                    )
                })
                .collect();
            // Zero lê no plural: "0 balões", não "0 balão". O CLDR do português põe o 0 em
            // `one`; a variação `zero` o iOS usa para 0 em qualquer língua.
            if !forms.contains_key("zero")
                && let Some(other) = forms.get("other")
            {
                variations.insert("zero".to_owned(), json!({ "stringUnit": string_unit(other, key) }));
            }

            json!({ "variations": { "plural": Value::Object(variations) } })
        }
    }
}

fn string_unit(text: &str, key: &Key) -> Value {
    json!({
        "state": "translated",
        "value": render(text, &key.placeholders, SWIFT),
    })
}

// ---------------------------------------------------------------------------
// InfoPlist.strings
// ---------------------------------------------------------------------------

/// The permission prompts iOS shows before the app is even on screen. They are
/// read out of the bundle's Info.plist, which no string table can reach, so
/// these get the old `<tag>.lproj/InfoPlist.strings` treatment.
fn info_plist_strings(catalog: &Catalog, out: &Path) -> Result<()> {
    let groups: Vec<&Group> = catalog
        .groups
        .iter()
        .filter(|group| group.info_plist)
        .collect();

    if groups.is_empty() {
        return Ok(());
    }

    for locale in &catalog.locales {
        let mut body = String::from("/* Generated from i18n/. Do not edit. */\n");

        for group in &groups {
            for key in &group.keys {
                let id = format!("{}.{}", group.name, key.name);
                let Some(Message::Simple(text)) = locale.messages.get(&id) else {
                    continue;
                };

                push!(
                    body,
                    "\"{}\" = \"{}\";\n",
                    key.name,
                    escape_strings_value(&render(text, &key.placeholders, SWIFT))
                );
            }
        }

        let dir = out.join("InfoPlist").join(format!("{}.lproj", locale.tag));
        std::fs::create_dir_all(&dir).with_context(|| format!("creating {}", dir.display()))?;
        write(&dir.join("InfoPlist.strings"), &body)?;
    }

    Ok(())
}

fn escape_strings_value(text: &str) -> String {
    text.replace('\\', "\\\\").replace('"', "\\\"")
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

/// A value bound out of an enum variant, in the type the accessor declares.
///
/// The accessor takes `Int`, and the payload arrives as whatever width the Rust
/// field has — `u32` reaches Swift as `UInt32`, which is not `Int` and does not
/// convert on its own. Every integer type Swift has takes an `Int(_:)`
/// initializer, so the conversion is the same line whatever the field was.
///
/// Text needs none: a `String` payload is already the accessor's type. Wrapping it
/// anyway would produce `String(name)`, which compiles and says nothing.
fn convert(name: &str, kind: PlaceholderKind) -> String {
    match kind {
        PlaceholderKind::Int => format!("Int({name})"),
        PlaceholderKind::Text => name.to_owned(),
    }
}

const fn swift_type(kind: PlaceholderKind) -> &'static str {
    match kind {
        PlaceholderKind::Int => "Int",
        PlaceholderKind::Text => "String",
    }
}

/// What the source language says, so Xcode's quick help shows the actual copy
/// next to the accessor rather than just its key.
fn source_preview(catalog: &Catalog, id: &str) -> String {
    let message = catalog
        .source()
        .messages
        .get(id)
        .expect("validated while loading");

    match message {
        Message::Simple(text) => format!("`{text}`"),
        Message::Plural(forms) => {
            let rendered: Vec<String> = forms
                .iter()
                .map(|(category, text)| format!("{category}: `{text}`"))
                .collect();

            rendered.join(", ")
        }
    }
}

fn push_doc(out: &mut String, indent: usize, text: Option<&str>) {
    let Some(text) = text else { return };
    let pad = " ".repeat(indent);

    for line in text.lines() {
        push!(out, "{pad}/// {line}\n");
    }
}

/// `draftCard` -> `DraftCard`
fn type_name(name: &str) -> String {
    let mut chars = name.chars();

    chars
        .next()
        .map(|first| first.to_uppercase().collect::<String>() + chars.as_str())
        .unwrap_or_default()
}

/// Keys are already written the way Swift wants them, except the Info.plist ones.
fn member_name(name: &str) -> String {
    name.to_owned()
}

fn write(path: &Path, contents: &str) -> Result<()> {
    std::fs::write(path, contents).with_context(|| format!("writing {}", path.display()))
}

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;

    use super::*;
    use crate::catalog::Placeholder;

    fn catalog() -> Catalog {
        Catalog {
            groups: vec![
                Group {
                    name: "notice".to_owned(),
                    comment: Some("Unrequested things".to_owned()),
                    enum_type: Some("Match.Notice".to_owned()),
                    property: "text".to_owned(),
                    resource: false,
                    info_plist: false,
                    keys: vec![Key {
                        name: "startFailed".to_owned(),
                        comment: None,
                        placeholders: vec![],
                        plural: false,
                    }],
                },
                Group {
                    name: "explanation".to_owned(),
                    comment: None,
                    enum_type: None,
                    property: "explanation".to_owned(),
                    resource: false,
                    info_plist: false,
                    keys: vec![Key {
                        name: "lineCount".to_owned(),
                        comment: None,
                        placeholders: vec![Placeholder {
                            name: "count".to_owned(),
                            kind: PlaceholderKind::Int,
                        }],
                        plural: true,
                    }],
                },
            ],
            locales: vec![crate::catalog::Locale {
                tag: "en".to_owned(),
                messages: BTreeMap::from([
                    (
                        "notice.startFailed".to_owned(),
                        Message::Simple("Couldn't reach the match server".to_owned()),
                    ),
                    (
                        "explanation.lineCount".to_owned(),
                        Message::Plural(BTreeMap::from([
                            ("one".to_owned(), "%{count} line".to_owned()),
                            ("other".to_owned(), "%{count} lines".to_owned()),
                        ])),
                    ),
                ]),
            }],
            source_language: "en".to_owned(),
            // Swift does not care: `case .deleteDraft` is the same syntax
            // whether or not the variant carries a payload.
            data_variant_enums: std::collections::BTreeSet::new(),
        }
    }

    #[test]
    fn a_key_without_placeholders_is_a_property() {
        assert!(
            strings_file(&catalog())
                .contains("static var startFailed: String { localized(\"notice.startFailed\") }")
        );
    }

    #[test]
    fn a_key_with_placeholders_takes_them_as_arguments() {
        assert!(strings_file(&catalog()).contains(
            "static func lineCount(_ count: Int) -> String { localized(\"explanation.lineCount\", [count]) }"
        ));
    }

    #[test]
    fn a_group_bound_to_a_core_type_becomes_an_exhaustive_switch() {
        let swift = mappers_file(&catalog());

        assert!(swift.contains("extension Match.Notice {"));
        assert!(swift.contains("var text: String {"));
        assert!(swift.contains("case .startFailed: Str.Notice.startFailed"));
        // No `default:` — that is what makes a new Rust variant a build failure.
        assert!(!swift.contains("default:"));
    }

    #[test]
    fn a_variant_whose_copy_interpolates_it_binds_what_it_carries() {
        let catalog = Catalog {
            groups: vec![Group {
                name: "submitFailure".to_owned(),
                comment: None,
                enum_type: Some("ChallengeDetail.SubmitFailure".to_owned()),
                property: "message".to_owned(),
                resource: false,
                info_plist: false,
                keys: vec![Key {
                    name: "refused".to_owned(),
                    comment: None,
                    placeholders: vec![Placeholder {
                        name: "message".to_owned(),
                        kind: PlaceholderKind::Text,
                    }],
                    plural: false,
                }],
            }],
            locales: vec![],
            source_language: "en".to_owned(),
            data_variant_enums: std::collections::BTreeSet::new(),
        };

        assert!(
            mappers_file(&catalog)
                .contains("case .refused(let message): Str.SubmitFailure.refused(message)")
        );
    }

    /// A number comes out of the variant in whatever width the Rust field has —
    /// `u32` reaches Swift as `UInt32` — and the accessor takes `Int`. Without the
    /// conversion the generated file does not compile, which is how this was found.
    #[test]
    fn a_number_bound_out_of_a_variant_is_converted_to_what_the_accessor_takes() {
        let catalog = Catalog {
            groups: vec![Group {
                name: "queueBacklog".to_owned(),
                comment: None,
                enum_type: Some("Match.QueueState".to_owned()),
                property: "message".to_owned(),
                resource: false,
                info_plist: false,
                keys: vec![Key {
                    name: "critical".to_owned(),
                    comment: None,
                    placeholders: vec![Placeholder {
                        name: "count".to_owned(),
                        kind: PlaceholderKind::Int,
                    }],
                    plural: false,
                }],
            }],
            locales: vec![],
            source_language: "en".to_owned(),
            data_variant_enums: std::collections::BTreeSet::new(),
        };

        assert!(
            mappers_file(&catalog)
                .contains("case .critical(let count): Str.QueueBacklog.critical(Int(count))")
        );
    }

    #[test]
    fn plurals_become_catalog_variations() {
        let json: Value = serde_json::from_str(&string_catalog(&catalog()).unwrap()).unwrap();
        let entry = &json["strings"]["explanation.lineCount"]["localizations"]["en"];

        assert_eq!(
            entry["variations"]["plural"]["one"]["stringUnit"]["value"],
            "%1$lld line"
        );
        assert_eq!(
            entry["variations"]["plural"]["other"]["stringUnit"]["value"],
            "%1$lld lines"
        );
        assert_eq!(
            entry["variations"]["plural"]["zero"]["stringUnit"]["value"],
            "%1$lld lines",
            "zero lê no plural"
        );
    }
}
