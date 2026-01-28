"use client";

import { Copy, Check } from "lucide-react";
import { useState } from "react";

interface TerminalProps {
    title?: string;
    command: string;
    className?: string;
    copy?: boolean;
}

export function Terminal({
    title = "Terminal",
    command,
    className,
    copy = true,
}: TerminalProps) {
    const [copied, setCopied] = useState(false);

    const copyToClipboard = () => {
        navigator.clipboard.writeText(command);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <div
            className={`relative ${className ?? ""}`}
            style={{ backgroundColor: "rgb(13, 13, 13)" }}
        >
            <div
                className="flex bg-zinc-950 items-center justify-between px-4 py-3 border-b"
                style={{ borderColor: "rgb(38, 38, 38)" }}
            >
                <div className="flex items-center gap-2 flex-1">
                    <span className="text-sm text-neutral-500">{">"}</span>
                    <span className="text-sm text-neutral-500">{title}</span>
                </div>
                {copy && (
                    <button
                        onClick={copyToClipboard}
                        className="p-1 rounded hover:bg-neutral-800 transition-colors"
                        title="Copy to clipboard"
                    >
                        {copied ? (
                            <Check className="h-4 w-4 text-green-400" />
                        ) : (
                            <Copy className="h-4 w-4 text-neutral-500" />
                        )}
                    </button>
                )}
            </div>
            <div className="px-4 py-3 bg-black">
                <code className="text-sm text-green-400">{command}</code>
            </div>
        </div>
    );
}
