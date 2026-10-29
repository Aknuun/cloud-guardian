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
        'fade-out': { from: { opacity: '1' }, to: { opacity: '0' } },
        // scale/opacity only: transform is reserved for the dialog centering
        'dialog-in': { from: { opacity: '0', scale: '0.95' }, to: { opacity: '1', scale: '1' } },
        'dialog-out': { from: { opacity: '1', scale: '1' }, to: { opacity: '0', scale: '0.97' } },
      },
      animation: {
        'fade-in': 'fade-in 150ms ease',
        'fade-out': 'fade-out 120ms ease',
        'dialog-in': 'dialog-in 150ms ease',
        'dialog-out': 'dialog-out 120ms ease',
      },
    },
  },
  plugins: [],
}
