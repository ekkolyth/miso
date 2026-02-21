import path from 'path'
import nextra from 'nextra'

const withNextra = nextra({
    defaultShowCopyCode: true,
    latex: true,
})

export default withNextra({
    reactStrictMode: true,
    outputFileTracingRoot: path.join(__dirname, '../../'),
})
