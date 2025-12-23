## What is `tag_translations.yaml`

`tag_translations.yaml` contains English translations of tags.
Example:

```yaml
漫画: manga
かわいい: cute
美少女: beautiful
```

## Formatting tag_translations.yaml

```sh
yq --inplace --prettyPrint 'sort_keys(.)' i18n/data/tag_translations.yaml
```

## Find tags that are not present in tag_translations.yaml

```sh
cat /tmp/pixivfe/responses/* | jq --compact-output --slurpfile translations <(yq -o json '.' i18n/data/tag_translations.yaml) -s '
  ([.[].body.thumbnails.novel[].tags[]] | unique) - ($translations[0] | keys)
'
```

## Prompt for generating tagTranslation objects from the above output

Tip: Try to get a good amount of family-friendly tags when attempting to generate translations; too many NSFW tags can trip safety classifiers.

````md
```json # JSON array of tags
JSON_ARRAY_OF_MISSING_TAGS_GOES_HERE
```

# TAG TRANSLATION TASK

You are given a JSON array containing tags in various languages. Per the normalization rules provided, create YAML objects for each tag in this format, escaping keys and values as necessary:

```yaml
漫画: manga
かわいい: cute
美少女: beautiful
```

```md # Normalization rules
NORMALIZATION_RULES_GO_HERE
```

## Process

1. Identify tags requiring translation
2. Research meaning and context
3. Provide appropriate English translations
4. Format as YAML objects

Complete this step by step.
````

## Prompt for normalizing - planning

````md
```yaml # Tags as YAML
TAGS_YAML_GOES_HERE
```

You will be analyzing the YAML dataset containing tag translations from Japanese to English provided above. Your task is to analyze the translations for inconsistencies and provide guided suggestions for normalizing the English translations.

Follow these steps to complete the task:

1. Analyze the translations (INCLUDING, BUT NOT LIMITED TO):
   a. Check for consistency in capitalization across all translations.
   b. Look for any plural/singular inconsistencies.
   c. Identify any abbreviations or acronyms that may need explanation.
   d. Check for formatting consistency (e.g., use of hyphens, spaces).

2. Based on your analysis, provide suggestions for normalizing the translations. Consider the following (INCLUDING, BUT NOT LIMITED TO):
   a. Should all translations start with a capital letter?
   b. Should numbers be written as digits or spelled out?
   c. Should abbreviations be expanded or kept as is?
   d. How should spaces and hyphens be used consistently?

Remember to consider the context of these tags when making your suggestions. Aim for clarity, consistency, and user-friendliness in your suggestions for the normalized translations.
````

## Prompt for normalizing - executing

NOTE: LLMs tend to give up when having to output the entire tag_translations.yaml, likely better to break it down into smaller chunks and normalize them in separate sessions.

````md
```yaml # Tags as YAML
TAGS_YAML_GOES_HERE
```

```md # Normalization rules
NORMALIZATION_RULES_GO_HERE
```

Your task is to normalize the English translations in the `Tags as YAML` per the normalization rules above.

Remember to think about the task step by step.
````

## Normalization rules

Copy and paste these where needed.

```md
## Tag translation normalization rules

### Introduction

This document specifies the rules for normalizing translated tags to ensure consistency and accuracy. All translations MUST adhere to these guidelines.

### Capitalization

Tags MUST be formatted in sentence case. Proper nouns are an exception and MUST be formatted in title case.

Sentence case means only the first letter of a tag is capitalized. If a tag contains multiple semantic units separated by a delimiter like a slash `/`, sentence case MUST be applied independently to each unit.

- Examples: `Barefoot`, `Bad end`, `Cool girl`, `Love-hate`, `Bad end/Happy end`.

Title case means capitalizing words according to standard English conventions for titles. It MUST be used for official or widely accepted names of series, games, books, or other proper nouns.

- Examples: `Honkai Star Rail`, `My Youth Romantic Comedy Is Wrong, As I Expected`.

For tags that mix proper and common nouns, the proper noun MUST retain its title case, while the common noun part of the tag MUST be in sentence case.

- Example: The tag `Gundam pilot` is correct, as `Gundam` is a proper noun and `pilot` is a common noun.
- Example: The tag for `Honkai Star Railのキャラクター` (character from Honkai Star Rail) MUST be `Character in Honkai Star Rail`.

If a term's status as a proper noun is ambiguous, the translator SHOULD first use its knowledge base to resolve the ambiguity. If the term remains ambiguous after this analysis, it MUST default to sentence case.

### Acronyms and clarifications

Clarity SHOULD be provided for acronyms and domain-specific slang.

English acronyms MUST be expanded.

- Example: `POV` becomes `Point of view`.

Non-English acronyms and slang MUST be translated to their underlying meaning. Romanization MUST NOT be used as the primary translation. Romanization MAY be provided in square brackets `[]` for additional context. This rule applies to common nouns, concepts, and slang. Acronyms representing proper nouns are governed by the rules in the 'Proper nouns and numbers' section.

- Example: `NTR` MUST be translated to `Cuckoldry [from netorare]`.
- Example: `わからせ` MUST be translated to `Putting someone in their place`.

If an acronym's meaning is genuinely ambiguous due to a lack of descriptive context, it MUST be preserved as-is in the translation. An honorific attached to an ambiguous acronym MUST also be preserved.

- Example: The tag `AI` is ambiguous and MUST be translated as `AI`.
- Example: The tag `AIさん` is ambiguous and MUST be translated as `AI-san`.
- Example: The tag `AI暴走` is not ambiguous, as `暴走` (rampage) provides context. It MUST be translated as `AI rampage`.

Nuance SHOULD be integrated directly into the translation. Square brackets `[]` SHOULD be used sparingly. Their use is reserved for secondary information, such as the origin of an acronym, or for noting unresolved, distinct meanings of the original tag. Delimiters from the original tag, such as parentheses, MUST NOT be converted into square brackets.

- Example: `(駄)自信作` MUST be translated as `Self-proclaimed masterpiece (sarcastic)`.

### Structural formatting

The structural formatting of the original tag, including delimiters, MUST be preserved.

The presence or absence of spaces around slashes `/` MUST be preserved exactly as in the original tag.

- Example: `イケメン女子/イケメン妻` becomes `Handsome girl/Handsome wife`.

The formatting of hash symbols `#` MUST be preserved. The only exception is for tags that function as a list, consisting of two or more terms separated exclusively by hashes. In this specific case, a space MUST be prepended to each hash symbol.

- Example (List): `皮裤#紧身裤#瑜伽裤` becomes `Leather pants #Tight pants #Yoga pants`.
- Example (Other): `chapter#5` remains `chapter#5`.
- Example (Other): `C#` remains `C#`.

Seemingly redundant or semantically similar terms in the original tag MUST NOT be removed or combined. All terms MUST be translated to maintain fidelity to the source.

- Example: `乱交/群交` MUST be translated to `Orgy/Group sex`, not simplified to `Orgy`.

### Transformation tags

Tags ending in the Japanese suffix `～化`, which denotes transformation, MUST be translated using the format `[Noun] transformation`. The `[Noun]` component MUST be translated according to all other rules in this document, including capitalization for proper nouns or the use of established loanwords.

- Examples: `Dragon transformation`, `Loli transformation`, `Satoshi transformation`, `Otokonoko transformation`.

### Proper nouns and numbers

The translation of proper nouns such as titles, character names, or places MUST follow a strict order of precedence.

- Use the official English translation if one exists.
- Use the widely accepted fan translation if no official translation exists.
- Use romanization if neither an official nor a widely accepted fan translation is available.

When romanization is required, specific standards MUST be followed.

- Japanese: Modified Hepburn
- Chinese (Simplified & Traditional): Pinyin
- Korean: Revised Romanization

An exception is if the source tag itself contains a non-standard romanization; in that case, the original spelling MUST be preserved.

Numbers in tags MUST be represented as digits, not spelled-out words.

- Examples: `Late 20s`, `4 siblings`.

For tags indicating user counts, the format `[Category] with [Number]+ users` MUST be used.

- Example: `BL novel with 500+ users`.

This rule MUST NOT be applied to fixed, standalone idiomatic phrases where a number is symbolic. For descriptive phrases, even common ones, digits MUST be used.

- Example (Idiom): `一石二鳥` becomes `Killing two birds with one stone`.
- Example (Descriptive): `三年目の浮気` becomes `The 3-year affair`.
```
