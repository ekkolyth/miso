import type { MDXComponents } from 'mdx/types'
import { useMDXComponents as getThemeComponents } from 'nextra-theme-docs'
import { InstallCommand } from '@/components/InstallCommand'

export function useMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...getThemeComponents(),
    InstallCommand,
    ...components,
  }
}
