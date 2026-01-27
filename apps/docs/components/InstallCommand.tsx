"use client";

import { Copy, Check } from "lucide-react";
import { useState } from "react";

type PackageManager = "npm" | "bun" | "pnpm" | "yarn" | "go";

const tabOrder: PackageManager[] = ["npm", "bun", "pnpm", "yarn", "go"];

const installCommands: Record<PackageManager, string> = {
    npm: "npm install -g @ekkolyth/miso",
    bun: "bun add -g @ekkolyth/miso",
    pnpm: "pnpm add -g @ekkolyth/miso",
    yarn: "yarn global add @ekkolyth/miso",
    go: "go install github.com/ekkolyth/miso/apps/miso/cmd@latest",
};

export function InstallCommand() {
    const [selected, setSelected] = useState<PackageManager>("pnpm");
    const [copied, setCopied] = useState(false);

    const copyToClipboard = () => {
        navigator.clipboard.writeText(installCommands[selected]);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <div className="my-6 rounded-lg border border-neutral-800 bg-neutral-900 overflow-hidden">
            {/* Tabs */}
            <div className="flex gap-8 px-6 border-b border-neutral-800">
                {tabOrder.map((pm) => (
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
                        {installCommands[selected]}
                    </code>
                </div>
            </div>
        </div>
    );
}
