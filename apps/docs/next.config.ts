import path from 'node:path'
import nextra from 'nextra'

const withNextra = nextra({
    defaultShowCopyCode: true,
    latex: true,
    search: true,
})

export default withNextra({
    reactStrictMode: true,
    outputFileTracingRoot: path.join(__dirname, '../../'),
})
