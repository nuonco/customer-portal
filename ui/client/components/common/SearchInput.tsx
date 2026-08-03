import { forwardRef, useId, type InputHTMLAttributes } from 'react'
import { cn } from '@/utils/classnames'
import { Button } from './Button'
import { Icon } from './Icon'

export interface ISearchInput
  extends Omit<
    InputHTMLAttributes<HTMLInputElement>,
    'autoComplete' | 'onChange' | 'type' | 'value'
  > {
  label: string
  labelClassName?: string
  labelTextClassName?: string
  placeholder?: string
  onChange: (val: string) => void
  onClear?: () => void
  value: string
}

export const SearchInput = forwardRef<HTMLInputElement, ISearchInput>(
  (
    {
      className,
      label,
      labelClassName,
      labelTextClassName,
      placeholder,
      onChange,
      onClear,
      value,
      ...props
    },
    ref
  ) => {
    const generatedId = useId()
    const inputId = props.id ?? generatedId

    return (
      <div className={cn('w-fit', labelClassName)}>
        <label
          htmlFor={inputId}
          className={cn('mb-1 block text-sm font-medium text-text-primary', labelTextClassName)}
        >
          {label}
        </label>

        <div className="relative flex">
          <Icon
            variant="MagnifyingGlassIcon"
            className="text-cool-grey-500 dark:text-cool-grey-700 absolute top-2.5 left-2"
          />
          <input
            ref={ref}
            id={inputId}
            className={cn(
              'rounded-md pl-8 pr-3.5 py-1.5 h-9 font-sans md:min-w-80 border text-sm',
              'bg-white dark:bg-dark-grey-900 placeholder:text-cool-grey-500 dark:placeholder:text-cool-grey-700',
              className
            )}
            type="text"
            placeholder={placeholder}
            autoComplete="off"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            {...props}
          />
          {value ? (
            <Button
              type="button"
              className="p-0.5! h-fit! absolute top-1/2 right-1.5 -translate-y-1/2"
              variant="ghost"
              title="clear search"
              onClick={() => (onClear ? onClear() : onChange(''))}
            >
              <Icon variant="XCircleIcon" />
            </Button>
          ) : null}
        </div>
      </div>
    )
  }
)

SearchInput.displayName = 'SearchInput'
