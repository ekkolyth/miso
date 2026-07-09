import { readFileSync } from 'node:fs'
import path from 'node:path'
import { Terminal } from './terminal'

// canonical example configs live in apps/miso/examples — the single source of truth
const FILES = {
    simple: 'miso.example.simple.json',
    single: 'miso.example.json',
    monorepo: 'miso.example.monorepo.json',
} as const

// read verbatim (preserves the files' hand-formatted compact style) at build time.
// cwd is apps/docs under Vercel/next build, repo root under a workspace build — try both.
function readExample(file: string): string {
    const candidates = [
        path.join(process.cwd(), '..', 'miso', 'examples', file),
        path.join(process.cwd(), 'apps', 'miso', 'examples', file),
    ]
    for (const candidate of candidates) {
        try {
            return readFileSync(candidate, 'utf8').trimEnd()
        } catch {
            // try next candidate
        }
    }
    throw new Error(
        `config example not found: ${file} (looked in ${candidates.join(', ')})`
    )
}

type ConfigExampleProps = {
    name: keyof typeof FILES
    title?: string
}

export function ConfigExample({
    name,
    title = 'miso.json',
}: ConfigExampleProps) {
    return <Terminal title={title} config={readExample(FILES[name])} />
}
