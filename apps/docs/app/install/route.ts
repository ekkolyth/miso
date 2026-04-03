import fs from 'node:fs'
import path from 'node:path'

// process.cwd() is apps/docs/ at Next.js startup — go up two levels to repo root.
// Do NOT use __dirname here: in compiled App Router output, __dirname points to
// .next/server/app/install/ (not the source tree), so relative paths would fail.
const SCRIPT_PATH = path.join(process.cwd(), '../../scripts/install.sh')

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
