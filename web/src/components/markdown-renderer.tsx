"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeHighlight from "rehype-highlight";
import { Copy, Check } from "lucide-react";
import { useState, useCallback } from "react";

interface Props {
  content: string;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [text]);

  return (
    <button
      onClick={handleCopy}
      className="absolute top-2 right-2 p-1.5 rounded-md transition-colors opacity-0 group-hover:opacity-100"
      style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
      title="Copy"
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </button>
  );
}

export default function MarkdownRenderer({ content }: Props) {
  return (
    <div className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex, rehypeHighlight]}
        components={{
          pre({ children, ...props }) {
            // Extract text content for copy button
            let text = "";
            try {
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              const codeEl = children as any;
              if (typeof codeEl === "object" && codeEl?.props?.children) {
                text = String(codeEl.props.children).replace(/\n$/, "");
              }
            } catch {
              // ignore extraction errors
            }
            return (
              <div className="relative group">
                <CopyButton text={text} />
                <pre {...props} className="overflow-x-auto rounded-lg p-4 text-sm" style={{ background: "var(--bg-hover)" }}>
                  {children}
                </pre>
              </div>
            );
          },
          code({ className, children, ...props }) {
            const isInline = !className;
            if (isInline) {
              return (
                <code
                  className="px-1.5 py-0.5 rounded text-sm"
                  style={{ background: "var(--bg-hover)", color: "var(--accent)" }}
                  {...props}
                >
                  {children}
                </code>
              );
            }
            return (
              <code className={className} {...props}>
                {children}
              </code>
            );
          },
          table({ children, ...props }) {
            return (
              <div className="overflow-x-auto my-3">
                <table className="min-w-full border-collapse text-sm" style={{ borderColor: "var(--border)" }} {...props}>
                  {children}
                </table>
              </div>
            );
          },
          th({ children, ...props }) {
            return (
              <th className="border px-3 py-2 text-left font-semibold" style={{ borderColor: "var(--border)", background: "var(--bg-hover)" }} {...props}>
                {children}
              </th>
            );
          },
          td({ children, ...props }) {
            return (
              <td className="border px-3 py-2" style={{ borderColor: "var(--border)" }} {...props}>
                {children}
              </td>
            );
          },
          a({ children, href, ...props }) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="underline"
                style={{ color: "var(--accent)" }}
                {...props}
              >
                {children}
              </a>
            );
          },
          blockquote({ children, ...props }) {
            return (
              <blockquote
                className="border-l-4 pl-4 my-3 italic"
                style={{ borderColor: "var(--accent)", color: "var(--text-muted)" }}
                {...props}
              >
                {children}
              </blockquote>
            );
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
