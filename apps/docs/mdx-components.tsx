import type { MDXComponents } from "mdx/types";
import { useMDXComponents as getThemeComponents } from "nextra-theme-docs";
import { CommandSelect } from "@/components/command-select";

export function useMDXComponents(components?: MDXComponents): MDXComponents {
    return {
        ...getThemeComponents(),
        InstallCommand: CommandSelect,
        ...components,
    };
}
