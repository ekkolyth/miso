import type { MDXComponents } from "mdx/types";
import { useMDXComponents as getThemeComponents } from "nextra-theme-docs";
import { CommandSelect } from "@/components/command-select";
import Miso from "./components/miso";

export function useMDXComponents(components?: MDXComponents): MDXComponents {
    return {
        ...getThemeComponents(),
        CommandSelect,
        Miso,
        ...components,
    };
}
