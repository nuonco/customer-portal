import { useState } from 'react'
import { SearchInput } from './SearchInput'

export default { title: 'Common/SearchInput' }

export const Empty = () => {
  const [value, setValue] = useState('')
  return (
    <SearchInput
      label="Search"
      placeholder="Search..."
      value={value}
      onChange={setValue}
    />
  )
}

export const WithValue = () => {
  const [value, setValue] = useState('my search query')
  return (
    <SearchInput
      label="Search"
      placeholder="Search..."
      value={value}
      onChange={setValue}
    />
  )
}

export const CustomPlaceholder = () => {
  const [value, setValue] = useState('')
  return (
    <SearchInput
      label="Search installs"
      placeholder="Search installs..."
      value={value}
      onChange={setValue}
    />
  )
}
