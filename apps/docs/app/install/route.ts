import fs from 'node:fs'
import path from 'node:path'

const SCRIPT_PATH = path.join(process.cwd(), 'public/install.sh')

export function GET(): Response {
    const script = fs.readFileSync(SCRIPT_PATH, 'utf-8')
    return new Response(script, {
        status: 200,
        headers: {
            'Content-Type': 'text/plain; charset=utf-8',
            'Cache-Control': 'no-cache',
        },
    })
}
