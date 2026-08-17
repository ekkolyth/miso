import Image from 'next/image'
import { Anchor, Head, Search } from 'nextra/components'
import { GitHubIcon } from 'nextra/icons'
import { getPageMap } from 'nextra/page-map'
import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import 'nextra-theme-docs/style-prefixed.css'
import './globals.css'
import { Badge } from '@/components/ui/badge'
import misoPackage from '../../miso/package.json'

export const metadata = {
    title: 'Miso - The Agnostic Package Manager',
    description: 'Documentation for Miso, the agnostic package manager',
}

// const banner = (
//   <Banner storageKey="miso-banner">🎉 Miso documentation is now live!</Banner>
// );

// simple-icons npm mark
const NpmIcon = (props: React.ComponentProps<'svg'>) => (
    <svg viewBox='0 0 24 24' fill='currentColor' {...props}>
        <title>npm</title>
        <path d='M1.763 0C.786 0 0 .786 0 1.763v20.474C0 23.214.786 24 1.763 24h20.474c.977 0 1.763-.786 1.763-1.763V1.763C24 .786 23.214 0 22.237 0zM5.13 5.323l13.837.019-.009 13.836h-3.464l.01-10.382h-3.456L12.04 19.17H5.113z' />
    </svg>
)

const navbar = (
    <Navbar
        logo={
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                <Image src='/miso.png' alt='Miso' width={36} height={36} />
                <span className='font-bold mr-1'>miso.js</span>
                <Badge
                    variant='default'
                    className='bg-primary-900 font-bold text-primary-400 border-primary-400'
                >
                    v{misoPackage.version}
                </Badge>
            </div>
        }
    >
        <Anchor
            href='https://www.npmjs.com/package/@ekkolyth/miso'
            aria-label='Package on npm'
        >
            <NpmIcon height='22' />
        </Anchor>
        <Anchor
            href='https://github.com/ekkolyth/miso'
            aria-label='Project repository'
        >
            <GitHubIcon height='24' />
        </Anchor>
    </Navbar>
)

const footer = (
    <Footer className='bg-black py-4'>
        Copyright © {new Date().getFullYear()} Miso.
    </Footer>
)

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode
}) {
    return (
        <html lang='en' dir='ltr' suppressHydrationWarning>
            <Head backgroundColor={{ dark: '#0e0e0e', light: '#ffffff' }}>
                <link rel='icon' href='/favicon.ico' />
            </Head>
            <body>
                <Layout
                    // banner={banner}
                    navbar={navbar}
                    pageMap={await getPageMap()}
                    docsRepositoryBase='https://github.com/ekkolyth/miso/apps/docs'
                    footer={footer}
                    search={<Search />}
                >
                    {children}
                </Layout>
            </body>
        </html>
    )
}
