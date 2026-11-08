import { normalizeOCRMath } from "./markdownMath";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";
export default function SafeMarkdown({ text }: { text: string }) {
  return (
    <div className="max-h-[32rem] overflow-auto break-words rounded-md border p-4 text-sm leading-7 [&_table]:border-collapse [&_td]:border [&_td]:p-2 [&_th]:border [&_th]:p-2 [&_pre]:overflow-auto [&_pre]:rounded [&_pre]:bg-muted [&_pre]:p-3 [&_p]:my-2">
      <Markdown
        skipHtml
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[
          [rehypeKatex, { throwOnError: false, trust: false, strict: "warn" }],
        ]}
        components={{
          img: ({ alt }) => <span>[图片：{alt ?? "已省略外部图像"}]</span>,
          a: ({ children, href }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="underline"
            >
              {children}
            </a>
          ),
        }}
      >
        {normalizeOCRMath(text)}
      </Markdown>
    </div>
  );
}
