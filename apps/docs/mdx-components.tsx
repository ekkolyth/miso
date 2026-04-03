import type { MDXComponents } from 'mdx/types'
import { useMDXComponents as getThemeComponents } from 'nextra-theme-docs'
import { Callout } from 'nextra/components'
import { CommandSelect } from '@/components/command-select'
import { Terminal } from './components/terminal'
import { DocProp } from './components/doc-prop'
import { DocHeading } from './components/doc-heading'
import { Badge } from './components/ui/badge'
import Miso from './components/miso'

export function useMDXComponents(components?: MDXComponents): MDXComponents {
    return {
        ...getThemeComponents(),
        Callout,
        CommandSelect,
        Terminal,
        DocProp,
        DocHeading,
        Badge,
        Miso,
        h3: (props) => <DocHeading {...props} />,
        ...components,
    }
}
