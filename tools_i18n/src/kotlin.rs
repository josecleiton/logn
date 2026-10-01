//! Android output: string resources, typed accessors, and the same Core-enum
//! mappers the Apple side gets.
//!
//! The mappers are the reason this is generated rather than written: an
//! exhaustive `when` with no `else` is what makes a new variant in
//! `shared_core/src/match_engine.rs` a build failure in *every* shell instead of a blank
//! label in one.
//!
//! Matching a variant is where Kotlin needs more from us than Swift does. Swift
//! spells every case `.camelCase` regardless of whether it carries a payload,
//! but facet's Kotlin typegen emits two different shapes — an `enum class` when
//! every variant is a unit variant, a `sealed interface` when any one of them is
//! not — and they are matched with different syntax. Which shape a Core enum
//! takes comes from `[enums]` in `keys.toml`, because this crate does not depend
//! on `shared` and so cannot read the Rust type.

use std::collections::BTreeSet;
use std::path::Path;

use anyhow::{Context as _, Result};

use crate::catalog::{Catalog, KOTLIN, Message, PlaceholderKind, render};

/// `write!` into a `String` cannot fail, and saying so at every call site would
/// bury what the emitter is actually writing.
macro_rules! push {
    ($out:expr, $($arg:tt)*) => {{
        use std::fmt::Write as _;
        let _ = write!($out, $($arg)*);
    }};
}

const BANNER: &str = "// Generated from i18n/ by `just android/i18n`. Do not edit.\n\
                      //\n\
                      // The copy lives in i18n/locales/*.toml; the keys in i18n/keys.toml.\n";

pub fn emit(catalog: &Catalog, out: &Path, package: &str, core_package: &str) -> Result<()> {
    for locale in &catalog.locales {
        let language = base_language(&locale.tag).unwrap_or(&locale.tag);
        anyhow::ensure!(
            LANGUAGES_WHERE_TWO_IS_OTHER.contains(&language),
            "{}: `pluralQuantity` lê o 0 como {ZERO_AS_PLURAL}, e não sei se {ZERO_AS_PLURAL} é `other` nesta língua",
            locale.tag
        );
    }
    for locale in &catalog.locales {
        // The source language is the default any unsupported locale falls back to.
        let mut directories = vec![if locale.tag == catalog.source_language {
            "values".to_owned()
        } else {
            format!("values-{}", qualifier(&locale.tag))
        }];

        // A device set to plain `pt` should still get Portuguese, not English.
        if let Some(language) = base_language(&locale.tag)
            && locale.tag != catalog.source_language
            && !catalog.locales.iter().any(|other| other.tag == language)
        {
            directories.push(format!("values-{language}"));
        }

        for directory in directories {
            let dir = out.join("res").join(directory);
            std::fs::create_dir_all(&dir).with_context(|| format!("creating {}", dir.display()))?;
            write(&dir.join("strings.xml"), &resources(catalog, locale))?;
        }
    }

    // android:localeConfig: the languages the app offers in the system's per-app language
    // setting, and what `cmd locale set-app-locales` accepts.
    let dir = out.join("res").join("xml");
    std::fs::create_dir_all(&dir).with_context(|| format!("creating {}", dir.display()))?;
    write(&dir.join("locales_config.xml"), &locales_config(catalog))?;

    let dir = out
        .join("java")
        .join(package.replace('.', "/"))
        .join("i18n");
    std::fs::create_dir_all(&dir).with_context(|| format!("creating {}", dir.display()))?;
    write(
        &dir.join("Strings.kt"),
        &strings_file(catalog, package, core_package),
    )?;

    Ok(())
}

// ---------------------------------------------------------------------------
// res/values*/strings.xml
// ---------------------------------------------------------------------------

fn resources(catalog: &Catalog, locale: &crate::catalog::Locale) -> String {
    let mut out = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n");
    out.push_str("<!-- Generated from i18n/. Do not edit. -->\n<resources>\n");

    // The language the interface actually resolved to, which is what the Core has to be
    // told: a device set to French shows the source language, and must say so. It is the
    // Android side of `Bundle.main.preferredLocalizations` on Apple. Not marked
    // `translatable="false"`: it differs per locale on purpose.
    push!(
        out,
        "    <string name=\"i18n_locale\">{}</string>\n",
        locale.tag
    );

    // Info.plist descriptions are an Apple concept; Android asks for permissions
    // without a reason string.
    for group in catalog.groups.iter().filter(|group| !group.info_plist) {
        push!(
            out,
            "\n    <!-- {} -->\n",
            group.comment.as_deref().unwrap_or(&group.name)
        );

        for key in &group.keys {
            let id = format!("{}.{}", group.name, key.name);
            let name = resource_name(&group.name, &key.name);
            let message = locale.messages.get(&id).expect("validated while loading");

            match message {
                Message::Simple(text) => {
                    push!(
                        out,
                        "    <string name=\"{name}\">{}</string>\n",
                        escape(&render(text, &key.placeholders, KOTLIN))
                    );
                }
                Message::Plural(forms) => {
                    push!(out, "    <plurals name=\"{name}\">\n");
                    for (category, text) in with_many(&locale.tag, forms) {
                        push!(
                            out,
                            "        <item quantity=\"{category}\">{}</item>\n",
                            escape(&render(text, &key.placeholders, KOTLIN))
                        );
                    }
                    out.push_str("    </plurals>\n");
                }
            }
        }
    }

    out.push_str("</resources>\n");
    out
}

/// Android needs `'`, `"`, `\` and `@`/`?` at the start escaped, and XML needs
/// `&` and `<` escaped whatever the platform thinks.
fn escape(text: &str) -> String {
    let escaped = text
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('\\', "\\\\")
        .replace('\'', "\\'")
        .replace('"', "\\\"");

    if escaped.starts_with('@') || escaped.starts_with('?') {
        format!("\\{escaped}")
    } else {
        escaped
    }
}

// ---------------------------------------------------------------------------
// Strings.kt
// ---------------------------------------------------------------------------

/// The name a namespaced Core enum is imported and written under.
///
/// `Match.Status` -> `MatchStatus`. The namespace is kept rather than
/// dropped because it is the only thing that makes the name unique: two domains may
/// each own a `Tab`, and Kotlin has no way to import both under one simple name.
fn receiver_name(enum_type: &str) -> String {
    enum_type.replace('.', "")
}

/// A placeholder's name as a Kotlin parameter, backticked when the word is one
/// Kotlin reserves.
///
/// `when` is the one this repository already has, and it is a *hard* keyword:
/// the accessor it appears in does not compile, so a placeholder nobody thought
/// twice about breaks the whole catalog for the Android shell. Swift takes the
/// same word without complaint, which is why the generator is where this belongs
/// rather than the copy.
fn parameter_name(name: &str) -> String {
    const RESERVED: [&str; 27] = [
        "as",
        "break",
        "class",
        "continue",
        "do",
        "else",
        "false",
        "for",
        "fun",
        "if",
        "in",
        "interface",
        "is",
        "null",
        "object",
        "package",
        "return",
        "super",
        "this",
        "throw",
        "true",
        "try",
        "typealias",
        "val",
        "var",
        "when",
        "while",
    ];

    if RESERVED.contains(&name) {
        format!("`{name}`")
    } else {
        name.to_owned()
    }
}

fn strings_file(catalog: &Catalog, package: &str, core_package: &str) -> String {
    let mut out = String::from(BANNER);
    push!(out, "\npackage {package}.i18n\n\n");
    out.push_str("import android.content.Context\n");
    push!(out, "import {package}.R\n");

    // Kotlin has no package imports, so every Core enum a mapper extends has to
    // be imported by name. `Match.Status` is namespace plus type: the
    // namespace is a sub-package of the generated types, and the type is what
    // the mapper writes. Deduplicated, because two groups can bind one type.
    //
    // **Aliased by namespace**, and that is not cosmetic. Two domains may each own
    // an enum of the same name — `Navigation.Tab` names the bottom bar and a screen
    // with its own faces would want the word too — and Kotlin refuses two imports
    // that resolve to one simple name. Without the alias the second such enum is a
    // build failure whose only fix is a synonym, and synonyms run out.
    let imports: BTreeSet<String> = catalog
        .groups_by_enum()
        .keys()
        .map(|enum_type| match enum_type.rsplit_once('.') {
            Some(_) => format!(
                "import {core_package}.{enum_type} as {}\n",
                receiver_name(enum_type)
            ),
            None => format!("import {core_package}.{enum_type}\n"),
        })
        .collect();
    for import in &imports {
        out.push_str(import);
    }
    out.push('\n');
    push!(
        out,
        "/**\n * Zero reads as plural (\"0 balões\"), as on iOS. CLDR puts 0 in `one` in Portuguese,\n \
         * and Android ignores a `zero` form the language does not have, so 0 is asked for as\n \
         * {ZERO_AS_PLURAL}, which is `other` in every language of the catalog.\n */\n\
         internal fun pluralQuantity(count: Int): Int = if (count == 0) {ZERO_AS_PLURAL} else count\n\n"
    );
    out.push_str(
        "/** Every user-facing string in the app, resolved against the string resources. */\n",
    );
    out.push_str("object Str {\n");

    let mut first = true;
    for group in catalog.groups.iter().filter(|group| !group.info_plist) {
        if !first {
            out.push('\n');
        }
        first = false;

        if let Some(comment) = &group.comment {
            push!(out, "    /** {comment} */\n");
        }
        push!(out, "    object {} {{\n", type_name(&group.name));

        for key in &group.keys {
            let name = resource_name(&group.name, &key.name);
            let mut parameters = String::new();
            let mut arguments = String::new();
            for placeholder in &key.placeholders {
                let identifier = parameter_name(&placeholder.name);
                push!(
                    parameters,
                    ", {identifier}: {}",
                    kotlin_type(placeholder.kind)
                );
                push!(arguments, ", {identifier}");
            }

            if key.plural {
                let count = key
                    .placeholders
                    .first()
                    .map_or_else(|| "0".to_owned(), |placeholder| placeholder.name.clone());

                push!(
                    out,
                    "        fun {}(context: Context{parameters}): String =\n            \
                     context.resources.getQuantityString(R.plurals.{name}, pluralQuantity({count}){arguments})\n",
                    member_name(&key.name)
                );
            } else {
                push!(
                    out,
                    "        fun {}(context: Context{parameters}): String =\n            \
                     context.getString(R.string.{name}{arguments})\n",
                    member_name(&key.name)
                );
            }
        }

        out.push_str("    }\n");
    }

    out.push_str("}\n");

    // The language picker's list. Emitted rather than restated in the shell so that
    // adding a language stays what ADR-0003 promises: one file in `i18n/locales/`.
    out.push_str(
        "\n/** Every language this build carries, source language first. */\nobject Languages {\n",
    );
    push!(
        out,
        "    const val SOURCE: String = \"{}\"\n\n",
        catalog.source_language
    );
    out.push_str("    val TAGS: List<String> = listOf(\n");
    let mut tags: Vec<&str> = catalog
        .locales
        .iter()
        .map(|locale| locale.tag.as_str())
        .collect();
    tags.sort_unstable_by_key(|tag| *tag != catalog.source_language);
    for tag in tags {
        push!(out, "        \"{tag}\",\n");
    }
    out.push_str("    )\n}\n");

    for (enum_type, groups) in catalog.groups_by_enum() {
        // `Match.Status` -> `MatchStatus`, which is the alias it was
        // imported under. Qualified rather than simple so that two domains may own
        // an enum of the same name — see the import block for why.
        let receiver = receiver_name(enum_type);
        let receiver = receiver.as_str();
        let sealed = catalog.has_data_variants(enum_type);

        for group in groups {
            push!(
                out,
                "\n/** {} */\nfun {receiver}.{}(context: Context): String = when (this) {{\n",
                group.comment.as_deref().unwrap_or(&group.name),
                member_name(&group.property)
            );

            for key in &group.keys {
                if sealed {
                    // A `sealed interface`: every variant is a nested type, so
                    // `is` is the only thing that matches one — including the
                    // unit variants, which are emitted as `data object`s.
                    push!(out, "    is {receiver}.{} -> ", variant_type(&key.name));
                } else {
                    // An `enum class`: the variant is a constant, not a type, so
                    // no `is`.
                    push!(out, "    {receiver}.{} -> ", variant_constant(&key.name));
                }

                // A variant that carries data whose copy interpolates it: the
                // placeholder is named after the field, and the smart cast on the
                // branch is what makes reading it legal. Without this the mapper
                // calls a two-argument accessor with one argument, which is a
                // compile error in the generated file rather than a wrong string —
                // the safeguard working, but only after somebody writes the key.
                //
                // An integer is narrowed to the accessor's `Int` on the way through:
                // the payload arrives as whatever width the Rust field has, and a
                // `u32` reaches Kotlin as `UInt`, which no overload of the accessor
                // takes. Text needs no conversion — a `String` payload is already
                // what the accessor declares.
                let mut arguments = String::new();
                for placeholder in &key.placeholders {
                    let name = parameter_name(&placeholder.name);
                    match placeholder.kind {
                        PlaceholderKind::Int => push!(arguments, ", this.{name}.toInt()"),
                        PlaceholderKind::Text => push!(arguments, ", this.{name}"),
                    }
                }

                push!(
                    out,
                    "Str.{}.{}(context{arguments})\n",
                    type_name(&group.name),
                    member_name(&key.name)
                );
            }

            // No `else`. The exhaustiveness is the point: a variant added to
            // `shared_core/src/match_engine.rs` has to break this build.
            out.push_str("}\n");
        }
    }

    out
}

// ---------------------------------------------------------------------------
// Naming
// ---------------------------------------------------------------------------

/// `draftCard` + `viewTranscript` -> `draft_card_view_transcript`
fn resource_name(group: &str, key: &str) -> String {
    format!("{}_{}", snake_case(group), snake_case(key))
}

fn snake_case(name: &str) -> String {
    let mut out = String::with_capacity(name.len() + 4);

    for (index, character) in name.chars().enumerate() {
        if character.is_uppercase() {
            if index > 0 {
                out.push('_');
            }
            out.extend(character.to_lowercase());
        } else {
            out.push(character);
        }
    }

    out
}

fn type_name(name: &str) -> String {
    let mut chars = name.chars();

    chars
        .next()
        .map(|first| first.to_uppercase().collect::<String>() + chars.as_str())
        .unwrap_or_default()
}

/// A variant of a `sealed interface`: a nested type, PascalCase as declared.
fn variant_type(name: &str) -> String {
    type_name(name)
}

/// A constant of an `enum class`.
///
/// facet's Kotlin emitter writes `name.to_uppercase()` on the Rust variant name,
/// which is *not* SCREAMING_SNAKE_CASE: `pausedByUser` becomes `PAUSEDBYUSER`,
/// not `PAUSED_BY_USER`. Inserting the underscores a Kotlin reader expects is the
/// most natural way to get this wrong.
fn variant_constant(name: &str) -> String {
    name.to_uppercase()
}

fn member_name(name: &str) -> String {
    name.to_owned()
}

const fn kotlin_type(kind: PlaceholderKind) -> &'static str {
    match kind {
        PlaceholderKind::Int => "Int",
        PlaceholderKind::Text => "String",
    }
}

/// `pt-BR` -> `pt-rBR`, the resource qualifier Android has used since before
/// BCP 47 tags were an option.
fn qualifier(tag: &str) -> String {
    match tag.split_once('-') {
        Some((language, region)) => format!("{language}-r{region}"),
        None => tag.to_owned(),
    }
}

fn base_language(tag: &str) -> Option<&str> {
    tag.split_once('-').map(|(language, _)| language)
}

/// The quantity a count of 0 is asked for with. Android picks the form by the language's
/// CLDR rule, which has no `zero` in Portuguese and puts 0 in `one`.
const ZERO_AS_PLURAL: u32 = 2;

/// Languages where [`ZERO_AS_PLURAL`] falls in `other`. A language outside the list (one
/// with a `two` form, say) has to be checked before it joins the catalog.
const LANGUAGES_WHERE_TWO_IS_OTHER: [&str; 5] = ["en", "es", "pt", "fr", "it"];

/// Languages whose CLDR plural rules have a `many` category (the "1 milhão de" form).
/// The catalog writes `one` and `other`, which is all Apple asks for; Android's lint
/// fails a `<plurals>` missing a category the language has.
const LANGUAGES_WITH_MANY: [&str; 5] = ["ca", "es", "fr", "it", "pt"];

/// The plural forms to emit: the catalog's, plus `many` as a copy of `other` where the
/// language has that category and the catalog does not spell it out. For every count
/// the app shows, `many` and `other` read the same.
fn with_many<'a>(
    tag: &str,
    forms: &'a std::collections::BTreeMap<String, String>,
) -> Vec<(&'a str, &'a String)> {
    let mut out: Vec<(&str, &String)> = forms.iter().map(|(k, v)| (k.as_str(), v)).collect();
    let language = base_language(tag).unwrap_or(tag);
    if LANGUAGES_WITH_MANY.contains(&language)
        && !forms.contains_key("many")
        && let Some(other) = forms.get("other")
    {
        out.push(("many", other));
    }
    out
}

fn locales_config(catalog: &Catalog) -> String {
    let mut out = String::from("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n");
    out.push_str("<!-- Generated from i18n/. Do not edit. -->\n");
    out.push_str("<locale-config xmlns:android=\"http://schemas.android.com/apk/res/android\">\n");
    for locale in &catalog.locales {
        push!(out, "    <locale android:name=\"{}\" />\n", locale.tag);
    }
    out.push_str("</locale-config>\n");
    out
}

fn write(path: &Path, contents: &str) -> Result<()> {
    std::fs::write(path, contents).with_context(|| format!("writing {}", path.display()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::catalog::{Group, Key, Locale, Placeholder};

    #[test]
    fn many_is_filled_from_other_where_the_language_has_it() {
        let forms = std::collections::BTreeMap::from([
            ("one".to_owned(), "1 balão".to_owned()),
            ("other".to_owned(), "%d balões".to_owned()),
        ]);
        let pt: Vec<_> = with_many("pt-BR", &forms).into_iter().map(|(k, _)| k).collect();
        assert_eq!(pt, ["one", "other", "many"]);
        assert_eq!(with_many("pt-BR", &forms)[2].1, "%d balões");
        let en: Vec<_> = with_many("en", &forms).into_iter().map(|(k, _)| k).collect();
        assert_eq!(en, ["one", "other"]);
    }

    #[test]
    fn zero_is_asked_for_as_plural() {
        let file = strings_file(&status_catalog(), "sh.logn.app", "sh.logn.core");
        assert!(file.contains("internal fun pluralQuantity(count: Int): Int = if (count == 0) 2 else count"));
    }

    #[test]
    fn a_language_where_two_is_not_other_is_refused() {
        let mut catalog = status_catalog();
        catalog.locales.push(Locale { tag: "ar".to_owned(), messages: std::collections::BTreeMap::new() });
        let error = emit(&catalog, Path::new("/nonexistent"), "sh.logn.app", "sh.logn.core").unwrap_err();
        assert!(error.to_string().contains("ar"), "{error}");
    }

    #[test]
    fn every_locale_says_which_language_it_is() {
        let catalog = status_catalog();
        for locale in &catalog.locales {
            let xml = resources(&catalog, locale);
            assert!(
                xml.contains(&format!(
                    "<string name=\"i18n_locale\">{}</string>",
                    locale.tag
                )),
                "{xml}"
            );
        }
        let config = locales_config(&catalog);
        for locale in &catalog.locales {
            assert!(config.contains(&format!("android:name=\"{}\"", locale.tag)));
        }
    }

    #[test]
    fn resource_names_are_snake_case() {
        assert_eq!(
            resource_name("draftCard", "viewTranscript"),
            "draft_card_view_transcript"
        );
        assert_eq!(
            resource_name("infoPlist", "JWTRefreshTokenTTL"),
            "info_plist_j_w_t_refresh_token_t_t_l"
        );
    }

    #[test]
    fn regional_tags_become_android_qualifiers() {
        assert_eq!(qualifier("pt-BR"), "pt-rBR");
        assert_eq!(qualifier("en"), "en");
    }

    #[test]
    fn escapes_what_android_reads_as_markup() {
        assert_eq!(escape("Don't & won't"), "Don\\'t &amp; won\\'t");
        assert_eq!(escape("@drafts"), "\\@drafts");
    }

    // -----------------------------------------------------------------------
    // Core-enum mappers
    //
    // Two shapes, two syntaxes, and the naming rule that is easiest to get
    // wrong. Everything here is about producing Kotlin that *compiles* — which
    // is what the generated `when` is for in the first place.
    // -----------------------------------------------------------------------

    #[test]
    fn an_enum_class_is_matched_on_qualified_upper_case_constants() {
        let file = strings_file(&status_catalog(), "sh.logn.app", "sh.logn.core");

        // Aliased by namespace, so a second domain owning a `Status` of its own
        // is a second import rather than a build failure with no fix but a synonym.
        assert!(
            file.contains("import sh.logn.core.Match.Status as MatchStatus"),
            "{file}"
        );
        assert!(
            file.contains(
                "fun MatchStatus.statusMessage(context: Context): String = when (this) {"
            ),
            "{file}"
        );
        assert!(
            file.contains("    MatchStatus.PAUSEDBYUSER -> Str.Status.pausedByUser(context)"),
            "{file}"
        );

        // The two halves of the old bug: `is` against an enum-class constant,
        // and the underscores a Kotlin reader expects but facet does not write.
        assert!(!file.contains("is MatchStatus."), "{file}");
        assert!(!file.contains("PAUSED_BY_USER"), "{file}");
    }

    #[test]
    fn a_sealed_interface_is_matched_on_nested_types() {
        let file = strings_file(&confirmation_catalog(), "sh.logn.app", "sh.logn.core");

        assert!(
            file.contains(
                "import sh.logn.core.Match.ConfirmationTarget as MatchConfirmationTarget"
            ),
            "{file}"
        );
        // `is`, and PascalCase — a data class and a data object are both types.
        assert!(
            file.contains(
                "    is MatchConfirmationTarget.DeleteDraft -> \
                 Str.ConfirmationTitle.deleteDraft(context)"
            ),
            "{file}"
        );
        assert!(!file.contains("DELETEDRAFT"), "{file}");
    }

    #[test]
    fn mappers_are_exhaustive_without_an_else() {
        // The whole reason the mappers are generated rather than written: a new
        // variant in shared_core/src/match_engine.rs has to break the Android build
        // instead of blanking a label at runtime.
        for catalog in [status_catalog(), confirmation_catalog()] {
            let file = strings_file(&catalog, "sh.logn.app", "sh.logn.core");
            assert!(!file.contains("else ->"), "{file}");
        }
    }

    /// A number comes out of the variant in whatever width the Rust field has —
    /// `u32` reaches Kotlin as `UInt` — and the accessor takes `Int`. Without the
    /// narrowing the generated file does not compile, which is how this was found.
    #[test]
    fn a_number_bound_out_of_a_variant_is_narrowed_to_what_the_accessor_takes() {
        let mut group = group("queueBacklog", "Match.QueueState", "message", &["critical"]);
        group.keys[0].placeholders = vec![Placeholder {
            name: "count".to_owned(),
            kind: PlaceholderKind::Int,
        }];

        let file = strings_file(
            &catalog(vec![group], &["Match.QueueState"]),
            "sh.logn.app",
            "sh.logn.core",
        );

        assert!(
            file.contains(
                "is MatchQueueState.Critical -> Str.QueueBacklog.critical(context, this.count.toInt())"
            ),
            "{file}"
        );
    }

    #[test]
    fn a_core_type_bound_by_two_groups_is_imported_once() {
        let file = strings_file(&two_groups_on_status(), "sh.logn.app", "sh.logn.core");

        assert_eq!(
            file.matches("import sh.logn.core.Match.Status")
                .count(),
            1,
            "{file}"
        );
        // Both mappers are still emitted, on the same receiver.
        assert!(
            file.contains("fun MatchStatus.statusMessage("),
            "{file}"
        );
        assert!(
            file.contains("fun MatchStatus.syncAccessibilityLabel("),
            "{file}"
        );
    }

    /// The reason the aliases exist: two domains owning an enum of the same name.
    ///
    /// `Navigation.Tab` names the bottom bar; a screen with faces of its own wants
    /// the word too. Without the alias Kotlin refuses the second import outright,
    /// and the only fix left is to rename one of them — which works exactly until
    /// the third screen wants it.
    #[test]
    fn two_domains_may_each_own_an_enum_of_the_same_name() {
        let catalog = Catalog {
            groups: vec![
                group("tabs", "Navigation.Tab", "label", &["calendar"]),
                group("challengePanel", "ChallengeDetail.Tab", "label", &["summary"]),
            ],
            ..two_groups_on_status()
        };

        let file = strings_file(&catalog, "sh.logn.app", "sh.logn.core");

        assert!(
            file.contains("import sh.logn.core.Navigation.Tab as NavigationTab"),
            "{file}"
        );
        assert!(
            file.contains("import sh.logn.core.ChallengeDetail.Tab as ChallengeDetailTab"),
            "{file}"
        );
        assert!(file.contains("fun NavigationTab.label("), "{file}");
        assert!(file.contains("fun ChallengeDetailTab.label("), "{file}");
    }

    #[test]
    fn enum_constants_upper_case_the_whole_name_without_underscores() {
        assert_eq!(variant_constant("idle"), "IDLE");
        assert_eq!(variant_constant("pausedByUser"), "PAUSEDBYUSER");
        assert_eq!(
            variant_constant("awaitingUnmeteredNetwork"),
            "AWAITINGUNMETEREDNETWORK"
        );
    }

    // -----------------------------------------------------------------------
    // Fixtures
    // -----------------------------------------------------------------------

    fn status_catalog() -> Catalog {
        catalog(
            vec![group(
                "status",
                "Match.Status",
                "statusMessage",
                &["idle", "pausedByUser"],
            )],
            &[],
        )
    }

    fn confirmation_catalog() -> Catalog {
        catalog(
            vec![group(
                "confirmationTitle",
                "Match.ConfirmationTarget",
                "title",
                &["discardMatch", "deleteDraft"],
            )],
            &["Match.ConfirmationTarget"],
        )
    }

    fn two_groups_on_status() -> Catalog {
        catalog(
            vec![
                group("status", "Match.Status", "statusMessage", &["idle"]),
                group(
                    "syncButton",
                    "Match.Status",
                    "syncAccessibilityLabel",
                    &["idle"],
                ),
            ],
            &[],
        )
    }

    fn group(name: &str, enum_type: &str, property: &str, keys: &[&str]) -> Group {
        Group {
            name: name.to_owned(),
            comment: None,
            enum_type: Some(enum_type.to_owned()),
            property: property.to_owned(),
            resource: false,
            info_plist: false,
            keys: keys
                .iter()
                .map(|key| Key {
                    name: (*key).to_owned(),
                    comment: None,
                    placeholders: vec![],
                    plural: false,
                })
                .collect(),
        }
    }

    fn catalog(groups: Vec<Group>, data_variant_enums: &[&str]) -> Catalog {
        let messages = groups
            .iter()
            .flat_map(|group| {
                group.keys.iter().map(move |key| {
                    (
                        format!("{}.{}", group.name, key.name),
                        Message::Simple(format!("copy for {}", key.name)),
                    )
                })
            })
            .collect();

        Catalog {
            groups,
            locales: vec![Locale {
                tag: "en".to_owned(),
                messages,
            }],
            source_language: "en".to_owned(),
            data_variant_enums: data_variant_enums
                .iter()
                .map(|name| (*name).to_owned())
                .collect(),
        }
    }
}
