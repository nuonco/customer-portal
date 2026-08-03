import { useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/common/Button'
import { Message } from '@/components/common/Message'
import { CheckboxInput } from '@/components/common/form/CheckboxInput'
import { Input } from '@/components/common/form/Input'
import { Select } from '@/components/common/form/Select'
import { Textarea } from '@/components/common/form/Textarea'
import { ModalBase } from '@/components/surfaces/Modal'
import { Text } from '@/components/common/Text'
import {
  createInstallLink,
  getOrgApps,
  getVendorAppInputConfig,
} from '@/lib/api/vendor/get-install-links'

type TInputField = {
  name: string
  display_name?: string
  description?: string
  type?: string
  required?: boolean
  default?: string
  sensitive?: boolean
  index?: number
}

type TInputGroup = {
  name?: string
  display_name?: string
  description?: string
  app_inputs?: TInputField[]
}

type TOrgApp = {
  id: string
  name: string
}

interface ICreateInstallLinkModal {
  orgId: string
  isOpen: boolean
  onClose: () => void
  onCreated: (linkId: string) => void
}

const installNamePattern = /^[a-zA-Z0-9 _-]+$/

export const CreateInstallLinkModal = ({
  orgId,
  isOpen,
  onClose,
  onCreated,
}: ICreateInstallLinkModal) => {
  const [apps, setApps] = useState<TOrgApp[]>([])
  const [selectedAppId, setSelectedAppId] = useState('')
  const [installName, setInstallName] = useState('')
  const [inputGroups, setInputGroups] = useState<TInputGroup[]>([])
  const [inputValues, setInputValues] = useState<Record<string, string>>({})
  const [isLoadingApps, setIsLoadingApps] = useState(false)
  const [isLoadingInputs, setIsLoadingInputs] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isOpen) return

    const loadApps = async () => {
      setIsLoadingApps(true)
      setError(null)
      try {
        const appList = await getOrgApps(orgId)
        setApps((appList ?? []).map((app) => ({ id: app.id, name: app.name })))
      } catch {
        setError('Failed to load apps')
      } finally {
        setIsLoadingApps(false)
      }
    }

    void loadApps()
  }, [isOpen, orgId])

  useEffect(() => {
    if (!selectedAppId) {
      setInputGroups([])
      setInputValues({})
      return
    }

    const loadInputConfig = async () => {
      setIsLoadingInputs(true)
      setError(null)
      try {
        const data = await getVendorAppInputConfig(orgId, selectedAppId)
        setInputGroups(data.input_config?.input_groups ?? [])
        setInputValues({})
      } catch {
        setInputGroups([])
      } finally {
        setIsLoadingInputs(false)
      }
    }

    void loadInputConfig()
  }, [orgId, selectedAppId])

  const selectedApp = useMemo(
    () => apps.find((app) => app.id === selectedAppId) ?? null,
    [apps, selectedAppId]
  )

  const resetAndClose = () => {
    setSelectedAppId('')
    setInstallName('')
    setInputGroups([])
    setInputValues({})
    setError(null)
    onClose()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!selectedApp) {
      setError('Please select an app')
      return
    }

    const trimmedName = installName.trim()
    if (!trimmedName) {
      setError('Please enter an install name')
      return
    }

    if (!installNamePattern.test(trimmedName)) {
      setError('Install name can only contain letters, numbers, spaces, dashes, and underscores')
      return
    }

    setIsSubmitting(true)
    try {
      const response = await createInstallLink(orgId, {
        app_id: selectedApp.id,
        app_name: selectedApp.name,
        name: trimmedName,
        inputs: inputValues,
      })
      onCreated(response.link.id)
      resetAndClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create install link')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!isOpen) return null

  return (
    <ModalBase
      isVisible={isOpen}
      onClose={resetAndClose}
      heading="Create Install Link"
      showFooter={false}
      size="lg"
      className="max-w-2xl"
    >
        <form className="space-y-6" onSubmit={(e) => void handleSubmit(e)}>
          <div>
            <Select
              id="create-install-link-app"
              required
              labelProps={{ labelText: 'Select Application *' }}
              value={selectedAppId}
              onChange={(e) => setSelectedAppId(e.target.value)}
              placeholder={isLoadingApps ? 'Loading apps...' : 'Select an app...'}
              options={apps.map((app) => ({
                value: app.id,
                label: app.name,
              }))}
            />
          </div>

          <div>
            <Input
              id="create-install-link-name"
              type="text"
              required
              labelProps={{ labelText: 'Install Name *' }}
              helperText="Letters, numbers, spaces, dashes, and underscores only."
              value={installName}
              onChange={(e) => setInstallName(e.target.value)}
              placeholder="e.g., customer-acme-prod"
            />
          </div>

          {isLoadingInputs ? (
            <p className="text-sm italic text-cool-grey-500 dark:text-cool-grey-400">
              Loading configuration...
            </p>
          ) : null}

          {inputGroups.length > 0 ? (
            <div className="mt-2 border-t border-cool-grey-200 pt-4 dark:border-dark-grey-600">
              <h4 className="mb-3 text-sm font-semibold text-cool-grey-800 dark:text-cool-grey-200">
                Vendor Configuration
              </h4>

              {inputGroups.map((group, groupIndex) => {
                const sortedInputs = [...(group.app_inputs ?? [])].sort(
                  (a, b) => (a.index ?? 0) - (b.index ?? 0)
                )

                return (
                  <div key={`${group.name ?? 'group'}-${groupIndex}`} className="mb-4">
                    <h5 className="mb-2 text-sm font-medium text-cool-grey-700 dark:text-cool-grey-300">
                      {group.display_name || group.name || 'Configuration'}
                    </h5>
                    {group.description ? (
                      <p className="mb-3 text-xs text-cool-grey-500 dark:text-cool-grey-400">
                        {group.description}
                      </p>
                    ) : null}

                    <div className="space-y-3">
                      {sortedInputs.map((input) => {
                        const isRequired = !!input.required
                        const label = `${input.display_name || input.name}${
                          isRequired ? ' *' : ''
                        }`
                        const value = inputValues[input.name] ?? input.default ?? ''

                        if (input.type === 'bool' || input.default === 'true' || input.default === 'false') {
                          const checked = value === 'true'
                          return (
                            <CheckboxInput
                              key={input.name}
                              id={`create-install-link-input-${input.name}`}
                              checked={checked}
                              onChange={(e) => {
                                setInputValues((prev) => ({
                                  ...prev,
                                  [input.name]: e.target.checked ? 'true' : 'false',
                                }))
                              }}
                              labelProps={{
                                className: 'items-start',
                                labelText: (
                                  <span className="flex-1">
                                    <span className="block text-sm font-medium text-cool-grey-700 dark:text-cool-grey-300">
                                      {label}
                                    </span>
                                    {input.description ? (
                                      <span className="mt-1 block text-xs text-cool-grey-500 dark:text-cool-grey-400">
                                        {input.description}
                                      </span>
                                    ) : null}
                                  </span>
                                ),
                              }}
                            />
                          )
                        }

                        if (input.type === 'json') {
                          return (
                            <Textarea
                              key={input.name}
                              id={`create-install-link-input-${input.name}`}
                                required={isRequired}
                              labelProps={{ labelText: label }}
                              helperText={input.description}
                              rows={3}
                                value={value}
                                placeholder={input.default || ''}
                                onChange={(e) => {
                                  setInputValues((prev) => ({
                                    ...prev,
                                    [input.name]: e.target.value,
                                  }))
                                }}
                            />
                          )
                        }

                        const type = input.type === 'number' ? 'number' : input.sensitive ? 'password' : 'text'

                        return (
                          <Input
                            key={input.name}
                            id={`create-install-link-input-${input.name}`}
                              type={type}
                              required={isRequired}
                            labelProps={{ labelText: label }}
                            helperText={input.description}
                              value={value}
                              autoComplete={type === 'password' ? 'off' : undefined}
                              placeholder={input.default || ''}
                              onChange={(e) => {
                                setInputValues((prev) => ({
                                  ...prev,
                                  [input.name]: e.target.value,
                                }))
                              }}
                          />
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          ) : null}

          {error ? (
            <Message>
              {error}
            </Message>
          ) : null}

          <div className="flex items-center justify-end gap-2 border-t border-cool-grey-200 pt-4 dark:border-dark-grey-700">
            <Button type="button" variant="secondary" onClick={resetAndClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {isSubmitting ? 'Creating...' : 'Create Link'}
            </Button>
          </div>
        </form>
    </ModalBase>
  )
}
