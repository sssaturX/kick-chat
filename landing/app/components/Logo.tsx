/** SaturX logo: arc + dot, uses currentColor */
export default function Logo({ className = 'w-8 h-8' }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 40 40"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-hidden
    >
      <circle
        cx="20"
        cy="20"
        r="14"
        stroke="currentColor"
        strokeWidth="5"
        strokeLinecap="round"
        strokeDasharray="60 32"
        strokeDashoffset="0"
        transform="rotate(-50 20 20)"
      />
      <circle cx="12" cy="10" r="3" fill="currentColor" />
    </svg>
  )
}
