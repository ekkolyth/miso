"use client";

import { Link, Check } from "lucide-react";
import { useState } from "react";

interface DocPropProps {
    prop: string;
    className?: string;
}

export function DocProp({ prop, className }: DocPropProps) {
    const [copied, setCopied] = useState(false);

    const copyLink = () => {
        const url = `${window.location.origin}${window.location.pathname}#${prop}`;
        navigator.clipboard.writeText(url);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <span
            id={prop}
            className={`mt-8 group inline-flex items-center gap-1.5 ${className ?? ""}`}
        >
            <code className="text-base px-1.5 py-0.5 rounded bg-neutral-800 text-neutral-200">
                {prop}
            </code>
            <button
                onClick={copyLink}
                className="opacity-0 group-hover:opacity-100 transition-opacity p-0.5 rounded hover:bg-neutral-800"
                title="Copy link to this prop"
            >
                {copied ? (
                    <Check className="h-3.5 w-3.5 text-green-400" />
                ) : (
                    <Link className="h-3.5 w-3.5 text-neutral-500" />
                )}
            </button>
        </span>
    );
}
