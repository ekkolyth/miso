/** @type {import('tailwindcss').Config} */
module.exports = {
    content: [
        './pages/**/*.{js,ts,jsx,tsx,mdx}',
        './components/**/*.{js,ts,jsx,tsx}',
        './app/**/*.{js,ts,jsx,tsx,mdx}',
        './content/**/*.{js,ts,jsx,tsx,mdx}',
        './mdx-components.tsx',
    ],
    theme: {
        extend: {
            colors: {
                primary: {
                    50: 'var(--x-color-primary-50)',
                    100: 'var(--x-color-primary-100)',
                    200: 'var(--x-color-primary-200)',
                    300: 'var(--x-color-primary-300)',
                    400: 'var(--x-color-primary-400)',
                    500: 'var(--x-color-primary-500)',
                    600: 'var(--x-color-primary-600)',
                    700: 'var(--x-color-primary-700)',
                    800: 'var(--x-color-primary-800)',
                    900: 'var(--x-color-primary-900)',
                    950: 'var(--x-color-primary-950)',
                },
            },
        },
    },
    plugins: [],
    darkMode: 'class',
}
