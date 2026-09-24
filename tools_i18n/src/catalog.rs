//! Reading and validating `i18n/`.
//!
//! The catalog is deliberately split in two: `keys.toml` says which strings
//! exist and what they interpolate, `locales/<tag>.toml` says what they read
//! like in one language. Keeping them apart is what makes it possible to answer
//! "is this language complete?" — with the copy and the schema in one file,
//! a missing translation is indistinguishable from a key that was never meant
//! to exist.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Write as _;
use std::path::Path;

use anyhow::{Context as _, Result, bail};
use serde::Deserialize;

/// Plural categories CLDR defines. A locale supplies the subset its language
/// actually distinguishes; `other` is the one every language has.
const PLURAL_CATEGORIES: [&str; 6] = ["zero", "one", "two", "few", "many", "other"];

// ---------------------------------------------------------------------------
// keys.toml
// ---------------------------------------------------------------------------

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SchemaFile {
    /// The language the others are translations of, and the one every locale the
    /// app was not translated into falls back to. Declared here rather than
    /// passed per shell: Apple and Android have to agree on the fallback, and
    /// two flags that must match are one flag too many.
    source_language: String,
    #[serde(default)]
    enums: BTreeMap<String, EnumMeta>,
    groups: BTreeMap<String, RawGroup>,
}

/// What a shell needs to know about a Core enum beyond the copy for its
/// variants.
#[derive(Debug, Default, Clone, Copy, Deserialize)]
#[serde(deny_unknown_fields)]
struct EnumMeta {
    /// At least one variant carries data.
    ///
    /// Declared rather than derived because this crate does not depend on
    /// `shared` and so cannot look at the Rust type. The cost is a fact stated
    /// twice; the safeguard is that getting it wrong fails the Kotlin compile
    /// rather than producing a wrong string, because the two shapes are matched
    /// with different syntax. See `kotlin::mappers`.
    #[serde(default)]
    data_variants: bool,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RawGroup {
    #[serde(default)]
    comment: Option<String>,
    #[serde(default, rename = "enum")]
    enum_type: Option<String>,
    #[serde(default)]
    property: Option<String>,
    #[serde(default)]
    resource: bool,
    #[serde(default)]
    info_plist: bool,
    keys: BTreeMap<String, RawKey>,
}

/// A key is either just a comment for the translator, or a table when it needs
/// placeholders or plural forms.
#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum RawKey {
    Comment(String),
    Meta(KeyMeta),
}

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
struct KeyMeta {
    #[serde(default)]
    comment: Option<String>,
    #[serde(default)]
    placeholders: Vec<Placeholder>,
    #[serde(default)]
    plural: bool,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Placeholder {
    pub name: String,
    #[serde(rename = "type")]
    pub kind: PlaceholderKind,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PlaceholderKind {
    Int,
    Text,
}

// ---------------------------------------------------------------------------
// locales/<tag>.toml
// ---------------------------------------------------------------------------

/// One translation: a single string, or one string per plural category.
#[derive(Debug, Clone, Deserialize)]
#[serde(untagged)]
pub enum Message {
    Simple(String),
    Plural(BTreeMap<String, String>),
}

impl Message {
    /// Every form this message carries, for checks that apply to all of them.
    fn forms(&self) -> Vec<&str> {
        match self {
            Self::Simple(text) => vec![text.as_str()],
            Self::Plural(forms) => forms.values().map(String::as_str).collect(),
        }
    }
}

// ---------------------------------------------------------------------------
// The validated catalog
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub struct Key {
    pub name: String,
    pub comment: Option<String>,
    pub placeholders: Vec<Placeholder>,
    pub plural: bool,
}

#[derive(Debug)]
pub struct Group {
    pub name: String,
    pub comment: Option<String>,
    /// A type from the generated `Match` module this group is the copy for.
    pub enum_type: Option<String>,
    /// Member generated on that type. Defaults to the group name.
    pub property: String,
    /// Also emit `LocalizedStringResource` accessors, which `AppIntents` needs.
    pub resource: bool,
    /// Belongs in `InfoPlist.strings` rather than the app's string table.
    pub info_plist: bool,
    pub keys: Vec<Key>,
}

#[derive(Debug)]
pub struct Locale {
    pub tag: String,
    /// Flattened as `group.key`, which is also how the key reads at runtime.
    pub messages: BTreeMap<String, Message>,
}

#[derive(Debug)]
pub struct Catalog {
    pub groups: Vec<Group>,
    pub locales: Vec<Locale>,
    /// The language the other files are translations of. Its copy is what a
    /// device with no matching locale falls back to.
    pub source_language: String,
    /// Core enums with at least one data-carrying variant, keyed by the
    /// `enum = "Match.X"` a group binds to. Read through
    /// [`Catalog::has_data_variants`].
    ///
    /// The flattened form of `[enums]` rather than the file's own shape: what
    /// the emitters need is the decided fact, and a set keeps the two from
    /// drifting if `EnumMeta` ever grows a second field.
    pub data_variant_enums: BTreeSet<String>,
}

impl Catalog {
    /// Reads `<root>/keys.toml` and every `<root>/locales/*.toml`, then checks
    /// them against each other. Every problem found is reported at once: a
    /// translator fixing a language wants the whole list, not the first line of it.
    pub fn load(root: &Path) -> Result<Self> {
        let schema_path = root.join("keys.toml");
        let schema: SchemaFile = read_toml(&schema_path)?;
        let source_language = schema.source_language;

        let groups = schema
            .groups
            .into_iter()
            .map(|(name, raw)| Group {
                property: raw.property.clone().unwrap_or_else(|| name.clone()),
                comment: raw.comment,
                enum_type: raw.enum_type,
                resource: raw.resource,
                info_plist: raw.info_plist,
                keys: raw
                    .keys
                    .into_iter()
                    .map(|(key_name, raw_key)| {
                        let meta = match raw_key {
                            RawKey::Comment(comment) => KeyMeta {
                                comment: Some(comment),
                                ..KeyMeta::default()
                            },
                            RawKey::Meta(meta) => meta,
                        };

                        Key {
                            name: key_name,
                            comment: meta.comment,
                            placeholders: meta.placeholders,
                            plural: meta.plural,
                        }
                    })
                    .collect(),
                name,
            })
            .collect::<Vec<_>>();

        let locales = read_locales(&root.join("locales"))?;

        if !locales.iter().any(|locale| locale.tag == source_language) {
            bail!(
                "source language `{source_language}` has no file in {}/locales",
                root.display()
            );
        }

        let catalog = Self {
            groups,
            locales,
            source_language,
            data_variant_enums: schema
                .enums
                .iter()
                .filter(|(_, meta)| meta.data_variants)
                .map(|(name, _)| name.clone())
                .collect(),
        };
        catalog.validate_enum_declarations(&schema.enums)?;
        catalog.validate()?;

        Ok(catalog)
    }

    /// An `[enums]` entry nobody binds is a rename left half-done: the group
    /// moved to a new type and this is still describing the shape of the old
    /// one, which is worse than saying nothing at all.
    fn validate_enum_declarations(&self, declared: &BTreeMap<String, EnumMeta>) -> Result<()> {
        let bound: BTreeSet<&str> = self.groups_by_enum().into_keys().collect();

        let orphans: Vec<&String> = declared
            .keys()
            .filter(|enum_type| !bound.contains(enum_type.as_str()))
            .collect();

        if orphans.is_empty() {
            return Ok(());
        }

        let mut report = String::from("keys.toml declares [enums] entries no group binds:");
        for enum_type in orphans {
            let _ = write!(report, "\n  - {enum_type}");
        }

        bail!(report)
    }

    fn validate(&self) -> Result<()> {
        let mut problems = Vec::new();

        let reserved_words = [
            "continue", "default", "in", "as", "is", "for", "while", "do", "if", "else", "switch", "case", "break", "return", "class", "struct", "enum", "func", "fun", "val", "var", "let", "guard", "defer", "typealias", "object", "when"
        ];

        let declared: BTreeMap<String, &Key> = self
            .groups
            .iter()
            .flat_map(|group| {
                for key in &group.keys {
                    if reserved_words.contains(&key.name.as_str()) {
                        problems.push(format!("key `{}.{}` uses reserved word `{}`", group.name, key.name, key.name));
                    }
                    if key.plural {
                        let ints = key.placeholders.iter().filter(|p| p.kind == PlaceholderKind::Int).count();
                        if key.placeholders.len() != 1 || ints != 1 {
                            problems.push(format!("key `{}.{}` is plural but does not have exactly one `int` placeholder", group.name, key.name));
                        }
                    }
                }

                group
                    .keys
                    .iter()
                    .map(move |key| (format!("{}.{}", group.name, key.name), key))
            })
            .collect();

        for locale in &self.locales {
            for id in declared.keys() {
                if !locale.messages.contains_key(id) {
                    problems.push(format!("{}: missing `{id}`", locale.tag));
                }
            }

            for (id, message) in &locale.messages {
                let Some(key) = declared.get(id) else {
                    problems.push(format!(
                        "{}: `{id}` is not declared in keys.toml",
                        locale.tag
                    ));
                    continue;
                };

                check_plural(&locale.tag, id, key, message, &mut problems);
                check_placeholders(&locale.tag, id, key, message, &mut problems);
            }
        }

        if problems.is_empty() {
            return Ok(());
        }

        let mut report = format!("the catalog has {} problem(s):", problems.len());
        for problem in problems {
            let _ = write!(report, "\n  - {problem}");
        }

        bail!(report)
    }

    /// The locale a value falls back to when a language does not carry the key
    /// — used for the doc comments on the generated accessors.
    pub fn source(&self) -> &Locale {
        self.locales
            .iter()
            .find(|locale| locale.tag == self.source_language)
            .expect("checked while loading")
    }

    /// Groups that are the copy for a Core type, keyed by that type, so one
    /// extension carries every member rather than one extension per member.
    /// Whether this Core enum becomes a Kotlin `sealed interface` rather than an
    /// `enum class` — which decides how a mapper is allowed to match a variant.
    ///
    /// Absent means every variant is a unit variant.
    pub fn has_data_variants(&self, enum_type: &str) -> bool {
        self.data_variant_enums.contains(enum_type)
    }

    pub fn groups_by_enum(&self) -> BTreeMap<&str, Vec<&Group>> {
        let mut by_type: BTreeMap<&str, Vec<&Group>> = BTreeMap::new();

        for group in &self.groups {
            if let Some(enum_type) = &group.enum_type {
                by_type.entry(enum_type.as_str()).or_default().push(group);
            }
        }

        by_type
    }
}

fn check_plural(tag: &str, id: &str, key: &Key, message: &Message, problems: &mut Vec<String>) {
    match (key.plural, message) {
        (true, Message::Simple(_)) => {
            problems.push(format!(
                "{tag}: `{id}` is plural, so it needs one form per category"
            ));
        }
        (false, Message::Plural(_)) => {
            problems.push(format!(
                "{tag}: `{id}` has plural forms but is not declared plural"
            ));
        }
        (true, Message::Plural(forms)) => {
            for category in forms.keys() {
                if !PLURAL_CATEGORIES.contains(&category.as_str()) {
                    problems.push(format!(
                        "{tag}: `{id}` has unknown plural category `{category}`"
                    ));
                }
            }
            if !forms.contains_key("other") {
                problems.push(format!("{tag}: `{id}` is missing the `other` plural form"));
            }
        }
        (false, Message::Simple(_)) => {}
    }
}

fn check_placeholders(
    tag: &str,
    id: &str,
    key: &Key,
    message: &Message,
    problems: &mut Vec<String>,
) {
    let declared: BTreeSet<&str> = key.placeholders.iter().map(|p| p.name.as_str()).collect();

    for form in message.forms() {
        let used = match scan_placeholders(form) {
            Ok(used) => used,
            Err(problem) => {
                problems.push(format!("{tag}: `{id}` {problem}"));
                continue;
            }
        };

        for name in used.difference(&declared) {
            problems.push(format!(
                "{tag}: `{id}` uses `%{{{name}}}`, which is not declared"
            ));
        }
        for name in declared.difference(&used) {
            problems.push(format!(
                "{tag}: `{id}` never uses the declared `%{{{name}}}`"
            ));
        }
    }
}

/// Placeholder names used by a piece of copy.
///
/// A bare `%` is an error rather than something to escape: no copy in the app
/// wants a literal percent sign, and letting one through would reach the
/// platform's format machinery as a specifier and read as garbage.
fn scan_placeholders(text: &str) -> Result<BTreeSet<&str>, String> {
    let mut names = BTreeSet::new();
    let mut rest = text;

    while let Some(start) = rest.find('%') {
        let after = &rest[start + 1..];

        let Some(inner) = after.strip_prefix('{') else {
            return Err("contains a literal `%`, which is reserved for placeholders".to_owned());
        };
        let Some(end) = inner.find('}') else {
            return Err("has a `%{` that is never closed".to_owned());
        };

        names.insert(&inner[..end]);
        rest = &inner[end + 1..];
    }

    Ok(names)
}

fn read_locales(dir: &Path) -> Result<Vec<Locale>> {
    let mut entries: Vec<_> = std::fs::read_dir(dir)
        .with_context(|| format!("reading {}", dir.display()))?
        .collect::<Result<Vec<_>, _>>()?
        .into_iter()
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "toml"))
        .collect();
    entries.sort();

    entries
        .iter()
        .map(|path| {
            let tag = path
                .file_stem()
                .and_then(|stem| stem.to_str())
                .with_context(|| format!("{} has no usable language tag", path.display()))?
                .to_owned();

            let groups: BTreeMap<String, BTreeMap<String, Message>> = read_toml(path)?;
            let messages = groups
                .into_iter()
                .flat_map(|(group, keys)| {
                    keys.into_iter()
                        .map(move |(key, message)| (format!("{group}.{key}"), message))
                })
                .collect();

            Ok(Locale { tag, messages })
        })
        .collect()
}

fn read_toml<T: serde::de::DeserializeOwned>(path: &Path) -> Result<T> {
    let text =
        std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;

    toml::from_str(&text).with_context(|| format!("parsing {}", path.display()))
}

// ---------------------------------------------------------------------------
// Rendering copy for a platform
// ---------------------------------------------------------------------------

/// How a platform spells a format specifier.
#[derive(Debug, Clone, Copy)]
pub struct Specifiers {
    /// Swift's `Int` is 64-bit, so `%d` would read half of one.
    pub int: &'static str,
    pub text: &'static str,
}

pub const SWIFT: Specifiers = Specifiers {
    int: "lld",
    text: "@",
};
pub const KOTLIN: Specifiers = Specifiers {
    int: "d",
    text: "s",
};

/// Rewrites `%{name}` into the platform's positional specifier.
///
/// Always positional (`%1$lld`), never bare: a translation is free to reorder
/// what a sentence puts first, and only positional arguments survive that.
pub fn render(text: &str, placeholders: &[Placeholder], spec: Specifiers) -> String {
    let mut out = String::with_capacity(text.len());
    let mut rest = text;

    while let Some(start) = rest.find("%{") {
        out.push_str(&rest[..start]);

        let inner = &rest[start + 2..];
        let end = inner.find('}').expect("validated while loading");
        let name = &inner[..end];

        let position = placeholders
            .iter()
            .position(|p| p.name == name)
            .expect("validated while loading");
        let kind = match placeholders[position].kind {
            PlaceholderKind::Int => spec.int,
            PlaceholderKind::Text => spec.text,
        };

        let _ = write!(out, "%{}${kind}", position + 1);
        rest = &inner[end + 1..];
    }

    out.push_str(rest);
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn key(plural: bool, placeholders: Vec<Placeholder>) -> Key {
        Key {
            name: "wordCount".to_owned(),
            comment: None,
            placeholders,
            plural,
        }
    }

    fn count_placeholder() -> Placeholder {
        Placeholder {
            name: "count".to_owned(),
            kind: PlaceholderKind::Int,
        }
    }

    #[test]
    fn renders_positional_specifiers_per_platform() {
        let placeholders = vec![count_placeholder()];

        assert_eq!(
            render("%{count} palavras", &placeholders, SWIFT),
            "%1$lld palavras"
        );
        assert_eq!(
            render("%{count} palavras", &placeholders, KOTLIN),
            "%1$d palavras"
        );
    }

    #[test]
    fn numbers_placeholders_by_declaration_order() {
        let placeholders = vec![
            Placeholder {
                name: "name".to_owned(),
                kind: PlaceholderKind::Text,
            },
            count_placeholder(),
        ];

        // Reordered by the translation; the positions follow the declaration.
        assert_eq!(
            render("%{count} de %{name}", &placeholders, SWIFT),
            "%2$lld de %1$@"
        );
    }

    #[test]
    fn rejects_a_placeholder_that_was_never_declared() {
        let mut problems = Vec::new();
        check_placeholders(
            "pt-BR",
            "explanation.lineCount",
            &key(false, vec![]),
            &Message::Simple("%{count} palavras".to_owned()),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("not declared"), "{problems:?}");
    }

    #[test]
    fn rejects_a_declared_placeholder_the_copy_never_uses() {
        let mut problems = Vec::new();
        check_placeholders(
            "en",
            "explanation.lineCount",
            &key(false, vec![count_placeholder()]),
            &Message::Simple("words".to_owned()),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("never uses"), "{problems:?}");
    }

    #[test]
    fn rejects_a_literal_percent() {
        let mut problems = Vec::new();
        check_placeholders(
            "en",
            "sync.progress",
            &key(false, vec![]),
            &Message::Simple("50% done".to_owned()),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("literal `%`"), "{problems:?}");
    }

    #[test]
    fn checks_every_plural_form_for_placeholders() {
        let mut problems = Vec::new();
        let forms = BTreeMap::from([
            ("one".to_owned(), "%{count} palavra".to_owned()),
            // The plural form forgot it.
            ("other".to_owned(), "palavras".to_owned()),
        ]);

        check_placeholders(
            "pt-BR",
            "explanation.lineCount",
            &key(true, vec![count_placeholder()]),
            &Message::Plural(forms),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("never uses"), "{problems:?}");
    }

    #[test]
    fn rejects_a_plural_key_given_a_single_form() {
        let mut problems = Vec::new();
        check_plural(
            "en",
            "explanation.lineCount",
            &key(true, vec![]),
            &Message::Simple("words".to_owned()),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(
            problems[0].contains("one form per category"),
            "{problems:?}"
        );
    }

    #[test]
    fn rejects_plural_forms_without_other() {
        let mut problems = Vec::new();
        let forms = BTreeMap::from([("one".to_owned(), "%{count} word".to_owned())]);

        check_plural(
            "en",
            "explanation.lineCount",
            &key(true, vec![count_placeholder()]),
            &Message::Plural(forms),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("`other`"), "{problems:?}");
    }

    fn catalog(locales: Vec<Locale>) -> Catalog {
        Catalog {
            groups: vec![Group {
                name: "notice".to_owned(),
                comment: None,
                enum_type: None,
                property: "notice".to_owned(),
                resource: false,
                info_plist: false,
                keys: vec![Key {
                    name: "startFailed".to_owned(),
                    comment: None,
                    placeholders: vec![],
                    plural: false,
                }],
            }],
            locales,
            source_language: "en".to_owned(),
            data_variant_enums: BTreeSet::new(),
        }
    }

    fn locale(tag: &str, messages: &[(&str, &str)]) -> Locale {
        Locale {
            tag: tag.to_owned(),
            messages: messages
                .iter()
                .map(|(id, text)| ((*id).to_owned(), Message::Simple((*text).to_owned())))
                .collect(),
        }
    }

    #[test]
    fn rejects_a_language_that_is_missing_a_key() {
        let catalog = catalog(vec![
            locale(
                "en",
                &[("notice.startFailed", "Couldn't reach the match server")],
            ),
            locale("pt-BR", &[]),
        ]);

        let error = catalog.validate().unwrap_err().to_string();

        assert!(
            error.contains("pt-BR: missing `notice.startFailed`"),
            "{error}"
        );
    }

    #[test]
    fn rejects_a_translation_of_a_key_nobody_declared() {
        let catalog = catalog(vec![locale(
            "en",
            &[
                ("notice.startFailed", "Couldn't reach the match server"),
                ("notice.stopFailed", "Left over from a key that was removed"),
            ],
        )]);

        let error = catalog.validate().unwrap_err().to_string();

        assert!(
            error.contains("`notice.stopFailed` is not declared"),
            "{error}"
        );
    }

    #[test]
    fn reports_every_problem_at_once() {
        let catalog = catalog(vec![
            locale("en", &[("notice.gone", "one")]),
            locale("pt-BR", &[]),
        ]);

        let error = catalog.validate().unwrap_err().to_string();

        // Two missing keys and one that is not declared.
        assert!(error.contains("3 problem(s)"), "{error}");
    }

    #[test]
    fn rejects_an_unknown_plural_category() {
        let mut problems = Vec::new();
        let forms = BTreeMap::from([
            ("plenty".to_owned(), "%{count} words".to_owned()),
            ("other".to_owned(), "%{count} words".to_owned()),
        ]);

        check_plural(
            "en",
            "explanation.lineCount",
            &key(true, vec![count_placeholder()]),
            &Message::Plural(forms),
            &mut problems,
        );

        assert_eq!(problems.len(), 1, "{problems:?}");
        assert!(problems[0].contains("plenty"), "{problems:?}");
    }
}
