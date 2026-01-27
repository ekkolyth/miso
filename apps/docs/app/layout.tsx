import { Footer, Layout, Navbar } from "nextra-theme-docs";
import { Banner, Head } from "nextra/components";
import { getPageMap } from "nextra/page-map";
import "nextra-theme-docs/style.css";
import "./globals.css";

export const metadata = {
    title: "Miso - The Agnostic Package Manager",
    description: "Documentation for Miso, the agnostic package manager",
};

// const banner = (
//   <Banner storageKey="miso-banner">🎉 Miso documentation is now live!</Banner>
// );

const navbar = (
    <Navbar
        logo={
            <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
                <img src="/miso.png" alt="Miso" width="36" height="36" />
                <span style={{ fontWeight: "bold" }}>miso.js</span>
            </div>
        }
        projectLink="https://github.com/ekkolyth/miso"
    />
);

const footer = (
    <Footer className="bg-black py-4">
        Copyright © {new Date().getFullYear()} Miso.
    </Footer>
);

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <html lang="en" dir="ltr" suppressHydrationWarning>
            <Head>
                <link rel="icon" href="/img/favicon.ico" />
            </Head>
            <body>
                <Layout
                    // banner={banner}
                    navbar={navbar}
                    pageMap={await getPageMap()}
                    docsRepositoryBase="https://github.com/ekkolyth/miso/apps/docs"
                    footer={footer}
                >
                    {children}
                </Layout>
            </body>
        </html>
    );
}
