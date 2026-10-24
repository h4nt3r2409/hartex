package agent

// User-facing output-language selection. hartex forces the natural
// language the agents show to the user into one of five languages: English,
// Korean, Chinese, Spanish, or Japanese. The agent "brain" (the reasoning body,
// section [A]) is now English like the rest of the codebase; only the surface
// the user reads follows this setting.

// OutputLanguage is the hook the server wires so the agent package can read the
// output language the operator selected (config's ARTEX_LANG or the runtime
// settings KV). A nil hook or an empty value falls back to defaultOutputLang.
// The server injects it the same way it injects PromptOverride.
var OutputLanguage func() string

// defaultOutputLang is used when the server has not wired the hook (for example,
// agent-package unit tests). It matches the product default, English. Korean,
// Chinese, Spanish, and Japanese are selected at runtime.
const defaultOutputLang = "en"

// SupportedOutputLangs is the list of selectable output-language codes.
var SupportedOutputLangs = []string{"en", "ko", "zh", "es", "ja"}

// normalizeLang narrows a code outside the supported list to the default.
func normalizeLang(code string) string {
	for _, c := range SupportedOutputLangs {
		if c == code {
			return code
		}
	}
	return defaultOutputLang
}

// resolveOutputLang returns the output-language code to apply. It prefers the
// hook the server wired and falls back to the default.
func resolveOutputLang() string {
	if OutputLanguage != nil {
		if code := OutputLanguage(); code != "" {
			return normalizeLang(code)
		}
	}
	return defaultOutputLang
}

// langPhrase is how the directive names a language the first time, with the
// native-script name in parentheses so the model cannot confuse it with another.
var langPhrase = map[string]string{
	"ko": "Korean (한국어)",
	"en": "English",
	"zh": "Chinese (简体中文)",
	"es": "Spanish (Español)",
	"ja": "Japanese (日本語)",
}

// langBare is the short language name used when the directive refers to the
// language again.
var langBare = map[string]string{
	"ko": "Korean",
	"en": "English",
	"zh": "Chinese",
	"es": "Spanish",
	"ja": "Japanese",
}
