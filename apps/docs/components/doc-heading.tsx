'use client'

import { Check, Link } from 'lucide-react'
import { useState } from 'react'

const COPY_RESET_DELAY = 2000

interface DocHeadingProps {
    children: React.ReactNode
    id?: string
}

export function DocHeading({ children, id }: DocHeadingProps) {
    const [copied, setCopied] = useState(false)

    const copyLink = () => {
        const url = `${window.location.origin}${window.location.pathname}#${id}`
        navigator.clipboard.writeText(url)
        setCopied(true)
        setTimeout(() => setCopied(false), COPY_RESET_DELAY)
    }

    return (
        <h3
            id={id}
            className='group mt-8 flex items-center gap-2 font-semibold text-xl'
        >
            <button
                type='button'
                onClick={copyLink}
                className='opacity-60 group-hover:opacity-100 transition-opacity p-0.5 rounded hover:bg-neutral-800'
                title='Copy link to this section'
            >
                {copied ? (
                    <Check className='h-4 w-4 text-green-400' />
                ) : (
                    <Link className='h-4 w-4 text-neutral-400' />
                )}
            </button>
            {children}
        </h3>
    )
}
