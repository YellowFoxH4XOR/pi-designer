// Model maker and transport/provider are different identities. Family wins for
// routed models (e.g. Claude on OpenRouter); the provider label stays visible.
export function modelIdentity(id = "", provider = ""): string | null {
  const model = id.toLowerCase();
  const route = provider.toLowerCase();
  if (/claude|anthropic|(?:^|[/_-])(?:opus|sonnet|haiku)/.test(model))
    return "claude";
  if (/deepseek/.test(model)) return "deepseek";
  if (/(?:^|[/_-])(?:gpt|o[134](?:[-/]|$)|chatgpt|codex)/.test(model))
    return "openai";
  if (/gemini/.test(model)) return "google-gemini";
  if (/opencode/.test(route)) return "opencode";
  if (/openai/.test(route)) return "openai";
  if (/anthropic/.test(route)) return "claude";
  if (/deepseek/.test(route)) return "deepseek";
  if (/google/.test(route)) return "google-gemini";
  if (/openrouter/.test(route)) return "openrouter";
  return null;
}
