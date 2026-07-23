import type { MDXComponents } from 'mdx/types'
import { Callout } from 'nextra/components'
import { useMDXComponents as getThemeComponents } from 'nextra-theme-docs'
import { CommandSelect } from '@/components/command-select'
import { ConfigExample } from './components/config-example'
import { DocHeading } from './components/doc-heading'
import { DocProp } from './components/doc-prop'
import Miso from './components/miso'
import { Terminal } from './components/terminal'
import { Badge } from './components/ui/badge'

export function useMDXComponents(components?: MDXComponents): MDXComponents {
    return {
        ...getThemeComponents(),
        Callout,
        CommandSelect,
        ConfigExample,
        Terminal,
        DocProp,
        DocHeading,
        Badge,
        Miso,
        h3: (props) => <DocHeading {...props} />,
        ...components,
    }
}
