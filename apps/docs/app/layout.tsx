import Image from 'next/image'
import { Head, Search } from 'nextra/components'
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
        projectLink='https://github.com/ekkolyth/miso'
    />
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
            <Head>
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
