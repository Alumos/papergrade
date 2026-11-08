// Convert Ollama TeX delimiters without enabling raw HTML.
export function normalizeOCRMath(text: string) {
  const slash = String.fromCharCode(92),
    tick = String.fromCharCode(96);
  return text
    .split(tick)
    .map((part, i) =>
      i % 2
        ? part
        : part
            .split(slash + "(")
            .join("$")
            .split(slash + ")")
            .join("$")
            .split(slash + "[")
            .join("$$")
            .split(slash + "]")
            .join("$$"),
    )
    .join(tick);
}
