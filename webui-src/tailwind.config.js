/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ['class', '[data-mode="dark"]'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        border: 'var(--ds-line)',
        input: 'var(--ds-line)',
        ring: 'var(--ds-brand)',
        background: 'var(--ds-bg)',
        foreground: 'var(--ds-text)',
        primary: { DEFAULT: 'var(--ds-brand)', foreground: '#ffffff' },
        secondary: { DEFAULT: 'var(--ds-panel)', foreground: 'var(--ds-text)' },
        muted: { DEFAULT: 'var(--ds-panel)', foreground: 'var(--ds-muted)' },
        accent: { DEFAULT: 'var(--ds-tint)', foreground: 'var(--ds-text)' },
        destructive: { DEFAULT: 'var(--ds-danger)', foreground: '#ffffff' },
        card: { DEFAULT: 'var(--ds-card)', foreground: 'var(--ds-text)' },
        popover: { DEFAULT: 'var(--ds-card)', foreground: 'var(--ds-text)' },
      },
      borderRadius: {
        lg: '12px',
        md: '10px',
        sm: '8px',
      },
      fontFamily: {
        sans: ['Vazirmatn', 'system-ui', '-apple-system', 'Segoe UI', 'Tahoma', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(16, 24, 40, 0.04)',
        pop: '0 12px 32px rgba(16, 24, 40, 0.14)',
      },
      keyframes: {
        'fade-in': { from: { opacity: '0' }, to: { opacity: '1' } },
        'slide-up': { from: { transform: 'translateY(16px)', opacity: '0.6' }, to: { transform: 'translateY(0)', opacity: '1' } },
      },
      animation: {
        'fade-in': 'fade-in 0.16s ease-out',
        'slide-up': 'slide-up 0.2s ease-out',
      },
    },
  },
  plugins: [],
}
