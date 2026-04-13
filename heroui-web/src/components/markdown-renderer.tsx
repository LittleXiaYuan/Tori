"use client";

import React, { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { Check, Copy } from "lucide-react";

interface MarkdownRendererProps {
  content: string;
}

function CodeBlock({ className, children }: { className?: string; children: React.ReactNode }) {
  const [copied, setCopied] = useState(false);
  const lang = className?.replace("hljs language-", "").replace("language-", "") || "code";
  const code = String(children).replace(/\n$/, "");

  const copy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="rounded-lg overflow-hidden my-2" style={{ background: "rgba(0,0,0,0.3)", border: "1px solid var(--yunque-border)" }}>
      <div className="flex items-center justify-between px-3 py-1.5" style={{ background: "rgba(255,255,255,0.03)" }}>
        <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>{lang}</span>
        <button onClick={copy} className="p-1 rounded transition-colors" style={{ color: "var(--yunque-text-muted)" }}>
          {copied ? <Check size={12} style={{ color: "var(--yunque-success)" }} /> : <Copy size={12} />}
        </button>
      </div>
      <pre className="px-3 py-2 overflow-x-auto custom-scrollbar !bg-transparent !m-0">
        <code className={`text-sm font-mono !bg-transparent ${className || ""}`}>{children}</code>
      </pre>
    </div>
  );
}

export default function MarkdownRenderer({ content }: MarkdownRendererProps) {
  if (!content) return null;

  return (
    <div className="markdown-body space-y-1">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          h1: ({ children }) => (
            <h1 className="text-xl font-bold mt-4 mb-2" style={{ color: "var(--yunque-text)" }}>{children}</h1>
          ),
          h2: ({ children }) => (
            <h2 className="text-lg font-semibold mt-4 mb-2" style={{ color: "var(--yunque-text)" }}>{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-base font-semibold mt-4 mb-2" style={{ color: "var(--yunque-text)" }}>{children}</h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-sm font-semibold mt-3 mb-1" style={{ color: "var(--yunque-text)" }}>{children}</h4>
          ),
          p: ({ children }) => (
            <p className="text-sm leading-relaxed" style={{ color: "var(--yunque-text)" }}>{children}</p>
          ),
          a: ({ href, children }) => (
            <a href={href} target="_blank" rel="noopener noreferrer" className="underline" style={{ color: "var(--yunque-accent)" }}>
              {children}
            </a>
          ),
          ul: ({ children }) => <ul className="ml-4 space-y-0.5 list-disc" style={{ color: "var(--yunque-accent)" }}>{children}</ul>,
          ol: ({ children }) => <ol className="ml-4 space-y-0.5 list-decimal" style={{ color: "var(--yunque-accent)" }}>{children}</ol>,
          li: ({ children }) => (
            <li className="text-sm" style={{ color: "var(--yunque-text)" }}>{children}</li>
          ),
          blockquote: ({ children }) => (
            <div className="pl-3 my-1" style={{ borderLeft: "2px solid var(--yunque-accent)" }}>
              <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{children}</div>
            </div>
          ),
          hr: () => <hr className="my-3" style={{ borderColor: "var(--yunque-border)" }} />,
          code: ({ className, children, ...props }) => {
            const isBlock = className?.includes("language-") || className?.includes("hljs");
            if (isBlock) {
              return <CodeBlock className={className}>{children}</CodeBlock>;
            }
            return (
              <code
                className="px-1.5 py-0.5 rounded text-xs font-mono"
                style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}
                {...props}
              >
                {children}
              </code>
            );
          },
          pre: ({ children }) => {
            const child = React.Children.toArray(children)[0];
            if (React.isValidElement(child) && child.type === "code") {
              return <>{children}</>;
            }
            return <pre className="overflow-x-auto">{children}</pre>;
          },
          table: ({ children }) => (
            <div className="overflow-x-auto my-2 rounded-lg" style={{ border: "1px solid var(--yunque-border)" }}>
              <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>{children}</table>
            </div>
          ),
          thead: ({ children }) => (
            <thead style={{ background: "rgba(255,255,255,0.03)" }}>{children}</thead>
          ),
          th: ({ children }) => (
            <th className="px-3 py-2 text-left font-medium text-xs" style={{ color: "var(--yunque-text-muted)", borderBottom: "1px solid var(--yunque-border)" }}>
              {children}
            </th>
          ),
          td: ({ children }) => (
            <td className="px-3 py-2 text-sm" style={{ color: "var(--yunque-text)", borderBottom: "1px solid var(--yunque-border)" }}>
              {children}
            </td>
          ),
          input: ({ type, checked, ...props }) => {
            if (type === "checkbox") {
              return (
                <input
                  type="checkbox"
                  checked={checked}
                  readOnly
                  className="mr-1.5 accent-[var(--yunque-accent)]"
                  {...props}
                />
              );
            }
            return <input type={type} {...props} />;
          },
          strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
          em: ({ children }) => <em>{children}</em>,
          del: ({ children }) => <del className="line-through" style={{ color: "var(--yunque-text-muted)" }}>{children}</del>,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
