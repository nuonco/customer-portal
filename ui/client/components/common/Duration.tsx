import { DateTime, Duration as LuxonDuration, type DurationUnits } from 'luxon'
import { Icon } from './Icon'
import { Text, type IText } from './Text'

export interface IDuration extends Omit<IText, 'role'> {
  beginTime?: string
  endTime?: string
  durationUnits?: DurationUnits
  format?: 'default' | 'timer'
  preserveSeconds?: boolean
  listStyle?: 'narrow' | 'short' | 'long'
  nanoseconds?: number
  unitDisplay?: 'narrow' | 'short' | 'long'
}

function formatHumanDuration(duration: LuxonDuration, options: {
  listStyle: 'narrow' | 'short' | 'long'
  preserveSeconds?: boolean
  unitDisplay: 'narrow' | 'short' | 'long'
}) {
  const humanized = duration.toHuman({
    listStyle: options.listStyle,
    unitDisplay: options.unitDisplay,
  })

  return options.preserveSeconds ? humanized.replaceAll(', ', ' ') : humanized
}

export const Duration = ({
  beginTime,
  endTime,
  durationUnits = [
    'years',
    'months',
    'days',
    'hours',
    'minutes',
    'seconds',
    'milliseconds',
  ],
  format = 'default',
  preserveSeconds = false,
  listStyle = 'narrow',
  nanoseconds,
  unitDisplay = 'narrow',
  ...props
}: IDuration) => {
  let duration: LuxonDuration | undefined

  if (typeof nanoseconds === 'number') {
    if (nanoseconds === 0) {
      return (
        <Text {...props}>
          <Icon variant="MinusIcon" />
        </Text>
      )
    }
    const milliseconds = Math.round(nanoseconds / 1e6)
    duration = LuxonDuration.fromMillis(milliseconds)
  } else if (beginTime) {
    const bt = DateTime.fromISO(beginTime)
    if (bt.isValid && bt.year > 1) {
      const et = endTime ? DateTime.fromISO(endTime) : DateTime.now()
      duration = et.diff(bt, durationUnits)
    }
  }

  return (
    <Text {...props} role="time">
      {duration?.isValid ? (
        format === 'timer' ? (
          duration.toFormat('T-hh:mm:ss:SS')
        ) : duration.as('seconds') < 1 ? (
          '< 1s'
        ) : duration.as('minutes') >= 1 ? (
          preserveSeconds
            ? formatHumanDuration(duration.rescale().set({ milliseconds: 0 }).rescale(), {
                listStyle,
                preserveSeconds,
                unitDisplay,
              })
            : formatHumanDuration(
                duration.rescale().set({ seconds: 0, milliseconds: 0 }).rescale(),
                {
                  listStyle,
                  unitDisplay,
                }
              )
        ) : (
          formatHumanDuration(duration.rescale().set({ milliseconds: 0 }).rescale(), {
            listStyle,
            preserveSeconds,
            unitDisplay,
          })
        )
      ) : (
        <Icon variant="MinusIcon" />
      )}
    </Text>
  )
}
