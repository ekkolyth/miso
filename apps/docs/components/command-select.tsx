"use client";

import { useState } from "react";
import { Terminal } from "./terminal";

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
        availableTabs[0] ?? "npm",
    );

    const currentCommand = props[selected] ?? "";

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
            <Terminal command={currentCommand} />
        </div>
    );
}
