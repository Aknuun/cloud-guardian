import type { CSSProperties } from 'react'

// Cloud Guardian mark: a shield holding a cloud, one blue accent, flat.
export function BrandMark({ size = 40, style }: { size?: number; style?: CSSProperties }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      role="img"
      aria-label="Cloud Guardian"
      style={{ display: 'block', borderRadius: size * 0.28, ...style }}
    >
      <defs>
        <linearGradient id="cgmk" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#2f6bff" />
          <stop offset="1" stopColor="#1f5eff" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="18" fill="url(#cgmk)" />
      <path
        d="M32 9.5 51 16.4v16.2c0 11.2-7.9 19.4-19 23.4-11.1-4-19-12.2-19-23.4V16.4L32 9.5Z"
        fill="#fff"
        fillOpacity=".97"
      />
      <path
        d="M24.2 36.6a4.4 4.4 0 0 1 .6-8.7 6.6 6.6 0 0 1 12.5-1.3 4.9 4.9 0 0 1 2.5 9.3l-.4.7H24.2Z"
        fill="#1f5eff"
      />
      <rect x="24.6" y="34.4" width="14.8" height="3.2" rx="1.6" fill="#1f5eff" />
    </svg>
  )
}
