import type { MDXComponents } from 'mdx/types'
import { useMDXComponents as getThemeComponents } from 'nextra-theme-docs'
import { Callout } from 'nextra/components'
import { CommandSelect } from '@/components/command-select'
import { Terminal } from './components/terminal'
import { DocProp } from './components/doc-prop'
import Miso from './components/miso'

export function useMDXComponents(components?: MDXComponents): MDXComponents {
    return {
        ...getThemeComponents(),
        Callout,
        CommandSelect,
        Terminal,
        DocProp,
        Miso,
        ...components,
    }
}
