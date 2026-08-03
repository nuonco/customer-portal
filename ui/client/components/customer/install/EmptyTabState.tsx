import { Card } from '@/components/common/Card'
import { Text } from '@/components/common/Text'

type TEmptyTabStateProps = {
  message: string
}

export function EmptyTabState({ message }: TEmptyTabStateProps) {
  return (
    <Card className="bg-surface">
      <Text variant="body" theme="neutral">
        {message}
      </Text>
    </Card>
  )
}