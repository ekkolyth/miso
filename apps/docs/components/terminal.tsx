'use client'

import { Check, Copy } from 'lucide-react'
import { useState } from 'react'

const COPY_RESET_DELAY = 2000

type TerminalProps = {
    title?: string
    className?: string
    copy?: boolean
} & (
    | { command: string; config?: never }
    | { config: string; command?: never }
)

function highlightJSON(text: string): React.ReactNode[] {
    // Tokenize line by line, apply colors to keys, strings, punctuation
    const lines = text.split('\n')
    return lines.map((line, i) => {
        const parts: React.ReactNode[] = []
        let remaining = line

        while (remaining.length > 0) {
            // Key: "someKey":
            const keyMatch = remaining.match(/^(\s*)("(?:[^"\\]|\\.)*")(\s*:\s*)/)
            if (keyMatch) {
                if (keyMatch[1]) parts.push(<span key={`${i}-ws-${parts.length}`} className='text-neutral-500'>{keyMatch[1]}</span>)
                parts.push(<span key={`${i}-key-${parts.length}`} className='text-blue-300'>{keyMatch[2]}</span>)
                parts.push(<span key={`${i}-colon-${parts.length}`} className='text-neutral-500'>{keyMatch[3]}</span>)
                remaining = remaining.slice(keyMatch[0].length)
                continue
            }

            // String value (possibly with | for union types)
            const strMatch = remaining.match(/^("(?:[^"\\]|\\.)*")/)
            if (strMatch) {
                parts.push(<span key={`${i}-str-${parts.length}`} className='text-green-400'>{strMatch[1]}</span>)
                remaining = remaining.slice(strMatch[0].length)
                continue
            }

            // Pipe separator for union types
            const pipeMatch = remaining.match(/^(\s*\|\s*)/)
            if (pipeMatch) {
                parts.push(<span key={`${i}-pipe-${parts.length}`} className='text-neutral-500'>{pipeMatch[1]}</span>)
                remaining = remaining.slice(pipeMatch[0].length)
                continue
            }

            // Braces, brackets, commas
            const punctMatch = remaining.match(/^([{}[\],])/)
            if (punctMatch) {
                parts.push(<span key={`${i}-punct-${parts.length}`} className='text-neutral-500'>{punctMatch[1]}</span>)
                remaining = remaining.slice(1)
                continue
            }

            // Numbers / booleans / null
            const literalMatch = remaining.match(/^(true|false|null|-?\d+(?:\.\d+)?)/)
            if (literalMatch) {
                parts.push(<span key={`${i}-lit-${parts.length}`} className='text-amber-400'>{literalMatch[1]}</span>)
                remaining = remaining.slice(literalMatch[0].length)
                continue
            }

            // Whitespace / anything else
            parts.push(<span key={`${i}-other-${parts.length}`} className='text-neutral-400'>{remaining[0]}</span>)
            remaining = remaining.slice(1)
        }

        const lineKey = `line-${i}-${line.slice(0, 8)}`
        return (
            <span key={lineKey}>
                {parts}
                {i < lines.length - 1 ? '\n' : ''}
            </span>
        )
    })
}

export function Terminal({
    title = 'Terminal',
    command,
    config,
    className,
    copy = true,
}: TerminalProps) {
    const [copied, setCopied] = useState(false)

    const displayText = config ?? command ?? ''

    const copyToClipboard = () => {
        navigator.clipboard.writeText(displayText)
        setCopied(true)
        setTimeout(() => setCopied(false), COPY_RESET_DELAY)
    }

    return (
        <div
            className={`relative ${className ?? ''}`}
            style={{ backgroundColor: 'rgb(13, 13, 13)' }}
        >
            <div
                className='flex bg-zinc-950 items-center justify-between px-4 py-3 border-b'
                style={{ borderColor: 'rgb(38, 38, 38)' }}
            >
                <div className='flex items-center gap-2 flex-1'>
                    <span className='text-sm text-neutral-500'>{'>'}</span>
                    <span className='text-sm text-neutral-500'>{title}</span>
                </div>
                {copy && (
                    <button
                        type='button'
                        onClick={copyToClipboard}
                        className='p-1 rounded hover:bg-neutral-800 transition-colors'
                        title='Copy to clipboard'
                    >
                        {copied ? (
                            <Check className='h-4 w-4 text-green-400' />
                        ) : (
                            <Copy className='h-4 w-4 text-neutral-500' />
                        )}
                    </button>
                )}
            </div>
            <div className='px-4 py-3 bg-black'>
                <code className='text-sm whitespace-pre font-mono'>
                    {config ? highlightJSON(displayText) : (
                        <span className='text-green-400'>{displayText}</span>
                    )}
                </code>
            </div>
        </div>
    )
}
