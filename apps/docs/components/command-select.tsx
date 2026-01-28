"use client";

import { Copy, Check } from "lucide-react";
import { useState } from "react";

type PackageManager = "miso" | "npm" | "bun" | "pnpm" | "yarn" | "go";

const tabOrder: PackageManager[] = ["miso", "npm", "bun", "pnpm", "yarn", "go"];

interface CommandSelectProps {
    miso?: string;
    npm?: string;
    bun?: string;
    pnpm?: string;
    yarn?: string;
    go?: string;
}

export function CommandSelect(props: CommandSelectProps) {
    // Filter tabs to only show those with provided commands
    const availableTabs = tabOrder.filter((pm) => props[pm] !== undefined);
    
    // Default to first available tab
    const [selected, setSelected] = useState<PackageManager>(
        availableTabs[0] ?? "npm"
    );
    const [copied, setCopied] = useState(false);

    const currentCommand = props[selected] ?? "";

    const copyToClipboard = () => {
        navigator.clipboard.writeText(currentCommand);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    // Don't render if no commands provided
    if (availableTabs.length === 0) {
        return null;
    }

    return (
        <div className="my-6 rounded-lg border border-neutral-800 bg-neutral-900 overflow-hidden">
            {/* Tabs */}
            <div className="flex gap-8 px-6 bg-neutral-950 border-b border-neutral-800">
                {availableTabs.map((pm) => (
                    <button
                        key={pm}
                        onClick={() => setSelected(pm)}
                        className={`
              relative py-3 text-sm font-medium transition-colors
              ${
                  selected === pm
                      ? "text-white"
                      : "text-neutral-500 hover:text-neutral-300"
              }
            `}
                    >
                        {pm}
                        {selected === pm && (
                            <div
                                className="absolute bottom-0 left-0 right-0 h-[2px]"
                                style={{
                                    backgroundColor:
                                        "oklch(54.1% 0.281 293.009)",
                                }}
                            />
                        )}
                    </button>
                ))}
            </div>

            {/* Terminal */}
            <div
                className="relative"
                style={{ backgroundColor: "rgb(13, 13, 13)" }}
            >
                <div
                    className="flex bg-zinc-950 items-center justify-between px-4 py-3 border-b"
                    style={{ borderColor: "rgb(38, 38, 38)" }}
                >
                    <div className="flex items-center gap-2 flex-1">
                        <span className="text-sm text-neutral-500">{">"}</span>
                        <span className="text-sm text-neutral-500">
                            Terminal
                        </span>
                    </div>
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
                </div>
                <div className="px-4 py-3 bg-black">
                    <code className="text-sm text-green-400">
                        {currentCommand}
                    </code>
                </div>
            </div>
        </div>
    );
}
