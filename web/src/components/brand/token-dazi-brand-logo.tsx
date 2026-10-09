import { cn } from '@/lib/utils'

export const TOKEN_DAZI_LOGO = '/token-dazi-logo.svg'

type TokenDaziBrandLogoProps = {
  alt: string
  className?: string
  markClassName?: string
  wordmarkClassName?: string
}

export function TokenDaziBrandLogo(props: TokenDaziBrandLogoProps) {
  return (
    <span
      data-token-dazi-brand='lockup'
      className={cn('inline-flex items-center gap-2.5', props.className)}
    >
      <img
        src={TOKEN_DAZI_LOGO}
        alt={props.alt}
        className={cn(
          'h-[38px] w-[38px] shrink-0 object-contain min-[901px]:h-9 min-[901px]:w-9 min-[1481px]:h-10 min-[1481px]:w-10',
          props.markClassName
        )}
      />
      <span
        data-token-dazi-wordmark
        aria-hidden='true'
        className={cn(
          'text-foreground text-[26px] leading-none font-bold min-[901px]:text-2xl min-[1481px]:text-[28px]',
          props.wordmarkClassName
        )}
      >
        Token 搭子
      </span>
    </span>
  )
}
