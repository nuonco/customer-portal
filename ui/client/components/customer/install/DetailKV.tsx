import { Text } from '@/components/common/Text'

type TDetailKVProps = {
  label: string
  value: string
}

export function DetailKV({ label, value }: TDetailKVProps) {
  return (
    <div>
      <Text
        as="div"
        variant="subtext"
        weight="stronger"
        className="uppercase tracking-[0.08em] text-text-muted"
      >
        {label}
      </Text>
      <Text as="div" variant="body">
        {value || '-'}
      </Text>
    </div>
  )
}